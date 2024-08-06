package lxdbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"sync"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

type Backend struct {
	nvidiaRuntime bool

	imageLock        sync.Mutex
	currentDownloads map[string]*downloadOp
}

const (
	LxdSock     = "/var/snap/lxd/common/lxd/unix.socket"
	storagePool = "default"
)

var (
	defaultDevices = createDefaultDevices
)

func InstanceName(name string, project_id string) string {
	return fmt.Sprintf("%s-%s", name, project_id)
}

func ImageAlias(name string) string {
	return fmt.Sprintf("workshop-%s-%s", name, runtime.GOARCH)
}

func New() (workshop.Backend, error) {
	server := Backend{
		currentDownloads: make(map[string]*downloadOp),
	}

	srv, err := lxd.ConnectLXDUnixWithContext(context.Background(), LxdSock, nil)
	if err != nil {
		return nil, err
	}

	resources, err := srv.GetServerResources()
	if err != nil {
		return nil, err
	}

	// Check if nvidia card(s) are present as this requires additional
	// configuration for the GPU interfaces runtime passthrough.
	for _, card := range resources.GPU.Cards {
		if card.Nvidia != nil {
			server.nvidiaRuntime = true
			break
		}
	}

	return &server, nil
}

func (s *Backend) LaunchWorkshop(ctx context.Context, file *workshop.File) error {
	var err error
	var image *api.Image

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	userName, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key user not found")
	}

	// Check if we have the base image stored locally
	alias, _, err := conn.GetImageAlias(ImageAlias(file.Base))
	if err != nil {
		return err
	}

	image, _, err = conn.GetImage(alias.Target)
	if err != nil {
		return err
	}

	usr, err := workshop.LookupUsername(userName)
	if err != nil {
		return err
	}

	config, err := s.workshopConfig(projectId, usr.Uid, usr.Gid, file)
	if err != nil {
		return err
	}
	req := api.InstancesPost{
		InstancePut: api.InstancePut{
			Devices: defaultDevices(),
			Config:  config,
		},
		Name: InstanceName(file.Name, projectId),
		Type: api.InstanceType("container"),
		Source: api.InstanceSource{
			Type:        "image",
			Fingerprint: image.Fingerprint,
			Project:     LxdProjectName(userName),
		},
	}

	op, err := conn.CreateInstance(req)
	if err != nil {
		return err
	}

	if err = op.WaitContext(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Backend) updateInstanceState(conn lxd.InstanceServer, ctx context.Context, name, action string, force bool) error {
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	// Do nothing if the instance is already in the desired state
	if (inst.StatusCode == api.Running && action == "start") ||
		(inst.StatusCode == api.Stopped && action == "stop") {
		return nil
	}

	req := api.InstanceStatePut{
		Action:  action,
		Timeout: 45,
		Force:   force,
	}

	op, err := conn.UpdateInstanceState(inst.Name, req, etag)
	if err != nil {
		return err
	}

	return op.WaitContext(ctx)
}

func (s *Backend) StartWorkshop(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	if err = s.updateInstanceState(conn, ctx, name, "start", false); err != nil {
		return err
	}

	// Wait until system is up an running before returning
	// see: https://blog.simos.info/how-to-know-when-a-lxd-container-has-finished-starting-up/
	args := workshop.Execution{
		ExecArgs: workshop.ExecArgs{
			UserId:  0,
			GroupId: 0,
			Command: []string{
				"bash", "-eu", "-c", "while " +
					"[ \"$(systemctl is-system-running 2>/dev/null)\" != \"running\" ] && " +
					"[ \"$(systemctl is-system-running 2>/dev/null)\" != \"degraded\" ]; do :; done",
			},
			WorkDir: "/",
		},
	}

	exectx, err := s.execCommand(conn, ctx, name, &args)
	if err != nil {
		return err
	}

	return exectx.WaitExecution(ctx)
}

func (s *Backend) StopWorkshop(ctx context.Context, name string, force bool) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	return s.updateInstanceState(conn, ctx, name, "stop", force)
}

func (s *Backend) AddWorkshopConfig(ctx context.Context, name string, item *workshop.WorkshopConfigValue) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}
	inst.Config[item.Name] = item.Value
	op, err := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)
	if err != nil {
		return err
	}

	return op.WaitContext(ctx)
}

func (s *Backend) RemoveWorkshopConfig(ctx context.Context, name string, key string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	delete(inst.Config, key)
	op, err := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)
	if err != nil {
		return err
	}

	return op.Wait()
}

func (s *Backend) AddWorkshopDevice(ctx context.Context, name string, device workshop.Device) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}
	inst.Devices[device.Name] = device.Properties
	op, err := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)
	if err != nil {
		return err
	}

	return op.Wait()
}

func (s *Backend) RemoveWorkshopDevice(ctx context.Context, name string, device string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	delete(inst.Devices, device)
	op, _ := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *Backend) execCommand(conn lxd.InstanceServer, ctx context.Context, name string, args *workshop.Execution) (workshop.ExecContext, error) {
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return workshop.ExecContext{}, fmt.Errorf("context key project-id not found")
	}

	req := api.InstanceExecPost{
		Command:     args.Command,
		WaitForWS:   true,
		Interactive: args.Interactive,
		Environment: args.Environment,
		Width:       args.Width,
		Height:      args.Height,
		User:        uint32(args.UserId),
		Group:       uint32(args.GroupId),
		Cwd:         args.WorkDir,
	}

	done := make(chan bool)

	op, err := conn.ExecInstance(InstanceName(name, projectId), req, &lxd.InstanceExecArgs{
		Stdin:    args.Stdin,
		Stdout:   args.Stdout,
		Stderr:   args.Stderr,
		Control:  args.Control,
		DataDone: done,
	})
	if err != nil {
		return workshop.ExecContext{}, err
	}

	opmeta := op.Get()
	var env = map[string]string{}
	for k, v := range opmeta.Metadata["environment"].(map[string]any) {
		if value, ok := v.(string); ok {
			env[k] = value
		}
	}

	return workshop.ExecContext{
		Environment: env,
		WaitExecution: func(ctx context.Context) error {
			defer conn.Disconnect()

			if err := op.WaitContext(ctx); err != nil {
				return err
			}

			// waiting for any remaining data IO to be flushed LXD closes this channel
			// unconditionally right after the operation has exited, so it will not be
			// blocked if we are here
			<-done
			var status = int(op.Get().Metadata["return"].(float64))
			if status != 0 {
				return &workshop.ErrExec{Status: status}
			}
			return nil
		},
	}, nil
}

func (s *Backend) Exec(ctx context.Context, name string, args *workshop.Execution) (workshop.ExecContext, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return workshop.ExecContext{}, err
	}

	return s.execCommand(conn, ctx, name, args)
}

func (s *Backend) Workshop(ctx context.Context, name string) (*workshop.Workshop, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
	}

	var p *workshop.Project
	projects, err := s.Projects(ctx)
	if err != nil {
		return nil, err
	}

	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return nil, fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	idx := slices.IndexFunc(projects[user], func(p *workshop.Project) bool { return p.ProjectId == projectId })
	if idx == -1 {
		return nil, fmt.Errorf("project %q is not found", projectId)
	}
	p = projects[user][idx]

	inst, _, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return nil, workshop.ErrWorkshopNotFound
		}
		return nil, err
	}

	workshop, err := s.loadWorkshop(inst, p)
	if err != nil {
		return nil, err
	}

	return workshop, nil
}

func workshopFile(lxdConfig map[string]string) (*workshop.File, error) {
	var f workshop.File
	if yml, ok := lxdConfig[workshop.ConfigWorkshopFile]; ok {
		if err := yaml.Unmarshal([]byte(yml), &f); err != nil {
			return nil, err
		}
	}
	return &f, nil
}

func (b *Backend) loadWorkshop(inst *api.Instance, p *workshop.Project) (*workshop.Workshop, error) {
	base := inst.Config["image.os"] + "@" + inst.Config["image.version"]

	f, err := workshopFile(inst.Config)
	if err != nil {
		return nil, fmt.Errorf("cannot load workshop: %v", err)
	}

	c := map[string]sdk.Setup{}
	if buf, exist := inst.Config[workshop.ConfigWorkshopContent]; exist {
		if err := json.Unmarshal([]byte(buf), &c); err != nil {
			return nil, err
		}
	}

	return &workshop.Workshop{
		Backend: b,
		Project: p,
		Name:    workshop.WorkshopName(inst.Name),
		Base:    base,
		Running: inst.StatusCode == api.Running || inst.StatusCode == api.Ready,
		Content: c,
		File:    f,
	}, nil
}

func (s *Backend) filterLxdInstancesByConfig(conn lxd.InstanceServer, filter workshop.WorkshopConfigFilter) ([]api.Instance, error) {
	instances, err := conn.GetInstances(api.InstanceTypeContainer)
	if err != nil {
		return nil, err
	}

	toReturn := make([]api.Instance, 0, len(instances))
	for _, i := range instances {
		if filter(i.Config) {
			toReturn = append(toReturn, i)
		}
	}

	return toReturn, nil
}

func (s *Backend) ProjectWorkshops(ctx context.Context) ([]*workshop.File, []*workshop.Workshop, error) {
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return nil, nil, fmt.Errorf("context key project-id not found")
	}

	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return nil, nil, fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Disconnect()

	var p *workshop.Project
	projects, err := s.Projects(ctx)
	if err != nil {
		return nil, nil, err
	}

	idx := slices.IndexFunc(projects[user], func(p *workshop.Project) bool { return p.ProjectId == projectId })
	if idx == -1 {
		return nil, nil, fmt.Errorf("project %q is not found", projectId)
	}

	p = projects[user][idx]

	files, err := p.ReadWorkshops()
	// if the dir does not exist it does not mean there are no workshops. It
	// could be because the dir was removed with some workshops still operating
	// resulting in a missing-project error
	if err != nil && !osutil.IsDirNotExist(err) {
		return nil, nil, err
	}

	// get all the running workshops for this project
	instances, err := conn.GetInstances(api.InstanceTypeContainer)
	if err != nil {
		return nil, nil, err
	}

	var projectWorkshops []*workshop.Workshop
	for _, i := range instances {
		if i.Config[workshop.ConfigProjectId] == p.ProjectId {
			ws, err := s.loadWorkshop(&i, p)
			if err != nil {
				logger.Debugf("error loading workshop: %v", err)
				continue
			}
			projectWorkshops = append(projectWorkshops, ws)
		}
	}

	wsFiles, wsInstances := mergeInstancesAndFiles(files, projectWorkshops)
	return wsFiles, wsInstances, nil
}

// Examine the lists of project's workshop files and workshops. Returns two
// lists. The first has *only* the workshop files that do not have any launched
// workshops yet, the second contains workshops that are launched with or
// without an associated file.
func mergeInstancesAndFiles(f []*workshop.File, instances []*workshop.Workshop) ([]*workshop.File, []*workshop.Workshop) {
	files := make([]*workshop.File, len(f))
	copy(files, f)
	// Walk both lists from to build a list of workshops with their states
	for _, ws := range instances {
		finder := func(p *workshop.File) bool { return p.Name == ws.Name }
		idx := slices.IndexFunc(files, finder)
		if idx != -1 {
			/* Both a file and instance exist */
			files = slices.Delete(files, idx, idx+1)
		}
	}

	// At this point, files contain only inactive workshops and instances
	// contain the workshops that have workshop files available.
	return files, instances
}

func (s *Backend) RemoveWorkshop(ctx context.Context, name string) (err error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	// ignore possible errors (e.g. container is already stopped)
	ctxRemove := context.WithoutCancel(ctx)
	_ = s.updateInstanceState(conn, ctxRemove, name, "stop", true)

	op, err := conn.DeleteInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	if err = op.WaitContext(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Backend) WorkshopFs(ctx context.Context, name string) (workshop.WorkshopFs, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
	}

	sftp, err := conn.GetInstanceFileSFTP(InstanceName(name, projectId))
	if err != nil {
		return nil, err
	}

	return workshop.NewWorkshopFs(sftp), nil
}

func (s *Backend) CreateStateStorage(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	pid, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", workshop.ContextProjectId)
	}

	// Create the storage volume entry
	vol := api.StorageVolumesPost{}
	vol.Name = workshop.WorkshopStateVolumeName(name, pid)
	vol.Type = "custom"
	vol.ContentType = "filesystem"
	vol.Config = map[string]string{}

	return conn.CreateStoragePoolVolume(storagePool, vol)
}

func (s *Backend) DeleteStateStorage(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	pid, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", workshop.ContextProjectId)
	}

	return conn.DeleteStoragePoolVolume(storagePool, "custom", workshop.WorkshopStateVolumeName(name, pid))
}

func (s *Backend) LxdClient(ctx context.Context) (lxd.InstanceServer, error) {
	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return nil, fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	if srv, err := lxd.ConnectLXDUnixWithContext(ctx, LxdSock, nil); err != nil {
		return nil, err
	} else {
		if err = InitLxdProject(srv, user); err != nil {
			return nil, err
		}
		return srv.UseProject(LxdProjectName(user)), nil
	}
}

func createDefaultDevices() map[string]map[string]string {
	return map[string]map[string]string{
		"root":                 {"type": "disk", "pool": storagePool, "path": "/"},
		"workshop.network":     {"type": "nic", "network": "lxdbr0", "name": "eth0"},
		"workshop.socket":      {"type": "disk", "source": dirs.SocketPath + ".untrusted", "path": filepath.Join(dirs.WorkshopBaseDir, ".workshop.socket.untrusted")},
		"workshop.workshopctl": {"type": "disk", "source": filepath.Join(dirs.ExecDir, "workshopctl"), "path": "/usr/bin/workshopctl"},
	}
}

func (s *Backend) workshopConfig(projectId string, userid, groupid string, file *workshop.File) (map[string]string, error) {
	cloudInitConfig := `#cloud-config
users:
  - default
  - name: workshop
    primary_group: workshop
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: adm,cdrom,sudo,dip,plugdev,audio,netdev,lxd,video,render
    shell: /bin/bash
`

	f, err := yaml.Marshal(file)
	if err != nil {
		return map[string]string{}, nil
	}

	cfg := map[string]string{
		"raw.idmap":                fmt.Sprint("uid ", userid, " 1000\ngid ", groupid, " 1000"),
		"security.nesting":         "true",
		"user.workshop.project-id": projectId,
		"user.user-data":           cloudInitConfig,
		"user.workshop.file":       string(f),
	}

	if s.nvidiaRuntime {
		// nvidia.* properties must be set at launch as otherwise it requires a
		// container restart to take effect.
		cfg["nvidia.driver.capabilities"] = "all"
		cfg["nvidia.runtime"] = "true"
	}
	return cfg, nil
}

func FakeDefaultDevices(f func() map[string]map[string]string) func() {
	oldDefault := defaultDevices
	defaultDevices = f
	return func() { defaultDevices = oldDefault }
}
