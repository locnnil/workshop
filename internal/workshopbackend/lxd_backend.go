package workshopbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/gorilla/websocket"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
)

type ExecArgs struct {
	Command     []string
	UserId      int
	GroupId     int
	WorkDir     string
	Timeout     time.Duration
	Environment map[string]string
	Interactive bool
	Terminal    bool
	SplitStderr bool
	Width       int
	Height      int
}

type ExecControls struct {
	Stdin   io.ReadCloser
	Stdout  io.WriteCloser
	Stderr  io.WriteCloser
	Control func(conn *websocket.Conn)
}

type Execution struct {
	ExecArgs
	ExecControls
}

type ExecContext struct {
	Environment   map[string]string
	WaitExecution func(ctx context.Context) error
}

type LxdBackend struct {
	nvidiaRuntime bool
}

const (
	LxdSock = "/var/snap/lxd/common/lxd/unix.socket"
)

var (
	ConnectSimpleStreams = lxd.ConnectSimpleStreams
	LookupUsername       = user.Lookup
	NewProjectId         = allocateProjectId

	defaultDevices = createDefaultDevices
	imageServer    = "https://cloud-images.ubuntu.com/releases/"

	LxdConfigProjectId         = "user.workshop.project-id"
	LxdConfigWorkshopFile      = "user.workshop.file"
	LxdConfigWorkshopContent   = "user.workshop.content"
	LxdConfigProjectPathDevice = "workshop.project"
)

func New() (WorkshopBackend, error) {
	server := LxdBackend{}

	srv, err := lxd.ConnectLXDUnixWithContext(context.Background(), LxdSock, nil)
	if err != nil {
		return nil, err
	}

	resources, err := srv.GetServerResources()
	if err != nil {
		return nil, err
	}

	// check if nvidia card(s) are present as this requires additional
	// configuration for the GPU interfaces runtime passthrough
	for _, card := range resources.GPU.Cards {
		if card.Nvidia != nil {
			server.nvidiaRuntime = true
			break
		}
	}

	return &server, nil
}

func (s *LxdBackend) LaunchWorkshop(ctx context.Context, file *WorkshopFile) error {
	var err error
	var imageSrv lxd.ImageServer
	var image *api.Image

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	userName, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key user not found")
	}

	// Skip if the instance exists already.
	if _, _, err := conn.GetInstance(InstanceName(file.Name, projectId)); err == nil {
		return fmt.Errorf("workshop \"%s\" already exists", file.Name)
	}

	// Check if we have the base image stored locally
	if alias, _, err := conn.GetImageAlias(file.Base); err == nil {
		if image, _, err = conn.GetImage(alias.Target); err != nil {
			return err
		}
		imageSrv = conn
	} else {
		imageSrv, image, err = s.fetchRemoteImage(file.Base)
		if err != nil {
			return err
		}
	}

	usr, err := LookupUsername(userName)
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
	op, err := conn.CreateInstanceFromImage(imageSrv, *image, req)
	if err != nil {
		return err
	}

	if err = op.Wait(); err != nil {
		return err
	}

	_, _, err = conn.GetImageAlias(file.Base)
	if err != nil && !api.StatusErrorCheck(err, http.StatusNotFound) {
		return err
	} else if api.StatusErrorCheck(err, http.StatusNotFound) {
		if err = conn.CreateImageAlias(api.ImageAliasesPost{ImageAliasesEntry: api.ImageAliasesEntry{
			Name: file.Base,
			ImageAliasesEntryPut: api.ImageAliasesEntryPut{
				Target: image.Fingerprint,
			},
		}}); err != nil {
			return err
		}
	}
	return nil
}

func (s *LxdBackend) updateInstanceState(conn lxd.InstanceServer, ctx context.Context, name, action string, force bool) error {
	projectId, ok := ctx.Value(ContextProjectId).(string)
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

func (s *LxdBackend) StartWorkshop(ctx context.Context, name string) error {
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
	args := Execution{
		ExecArgs: ExecArgs{
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

func (s *LxdBackend) StopWorkshop(ctx context.Context, name string, force bool) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	return s.updateInstanceState(conn, ctx, name, "stop", force)
}

func (s *LxdBackend) AddWorkshopConfig(ctx context.Context, name string, item *WorkshopConfigValue) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(ContextProjectId).(string)
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

func (s *LxdBackend) RemoveWorkshopConfig(ctx context.Context, name string, key string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(ContextProjectId).(string)
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

func (s *LxdBackend) AddWorkshopDevice(ctx context.Context, name string, device Device) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}
	inst.Devices[device.Name()] = device.properties
	op, err := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)
	if err != nil {
		return err
	}

	return op.Wait()
}

func (s *LxdBackend) RemoveWorkshopDevice(ctx context.Context, name string, device string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(ContextProjectId).(string)
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

func (s *LxdBackend) execCommand(conn lxd.InstanceServer, ctx context.Context, name string, args *Execution) (ExecContext, error) {
	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return ExecContext{}, fmt.Errorf("context key project-id not found")
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
		return ExecContext{}, err
	}

	opmeta := op.Get()
	var env = map[string]string{}
	for k, v := range opmeta.Metadata["environment"].(map[string]any) {
		if value, ok := v.(string); ok {
			env[k] = value
		}
	}

	return ExecContext{
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
				return &ErrExec{Status: status}
			}
			return nil
		},
	}, nil
}

func (s *LxdBackend) Exec(ctx context.Context, name string, args *Execution) (ExecContext, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return ExecContext{}, err
	}

	return s.execCommand(conn, ctx, name, args)
}

func (s *LxdBackend) Workshop(ctx context.Context, name string) (*Workshop, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
	}

	var p *Project
	projects, err := s.Projects(ctx)
	if err != nil {
		return nil, err
	}

	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return nil, fmt.Errorf("context key %s not found", ContextUser)
	}

	idx := slices.IndexFunc(projects[user], func(p *Project) bool { return p.ProjectId == projectId })
	if idx == -1 {
		return nil, fmt.Errorf("project %q is not found", projectId)
	}
	p = projects[user][idx]

	inst, _, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return nil, ErrWorkshopNotFound
		}
		return nil, err
	}

	workshop, err := s.loadWorkshop(inst, p)
	if err != nil {
		return nil, err
	}

	return workshop, nil
}

func installedContent(lxdConfig map[string]string) (map[string]sdk.Setup, error) {
	c := make(map[string]sdk.Setup)
	if sdks, ok := lxdConfig[LxdConfigWorkshopContent]; ok {
		if err := json.Unmarshal([]byte(sdks), &c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func workshopFile(lxdConfig map[string]string) (*WorkshopFile, error) {
	var f WorkshopFile
	if yml, ok := lxdConfig[LxdConfigWorkshopFile]; ok {
		if err := yaml.Unmarshal([]byte(yml), &f); err != nil {
			return nil, err
		}
	}
	return &f, nil
}

func (b *LxdBackend) loadWorkshop(inst *api.Instance, p *Project) (*Workshop, error) {
	base := inst.Config["image.os"] + "@" + inst.Config["image.version"]

	f, err := workshopFile(inst.Config)
	if err != nil {
		return nil, fmt.Errorf("cannot load workshop: %v", err)
	}

	content, err := installedContent(inst.Config)
	if err != nil {
		return nil, fmt.Errorf("cannot load workshop: %v", err)
	}

	return &Workshop{
		Name:    WorkshopName(inst.Name),
		backend: b,
		project: p,
		file:    f,
		running: inst.StatusCode == api.Running || inst.StatusCode == api.Ready,
		base:    base,
		content: content,
	}, nil
}

func (s *LxdBackend) filterLxdInstancesByConfig(conn lxd.InstanceServer, filter WorkshopConfigFilter) ([]api.Instance, error) {
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

func (s *LxdBackend) ProjectWorkshops(ctx context.Context) ([]*WorkshopFile, []*Workshop, error) {
	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, nil, fmt.Errorf("context key project-id not found")
	}

	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return nil, nil, fmt.Errorf("context key %s not found", ContextUser)
	}

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Disconnect()

	var p *Project
	projects, err := s.Projects(ctx)
	if err != nil {
		return nil, nil, err
	}

	idx := slices.IndexFunc(projects[user], func(p *Project) bool { return p.ProjectId == projectId })
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

	var projectWorkshops []*Workshop
	for _, i := range instances {
		if i.Config[LxdConfigProjectId] == p.ProjectId {
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
func mergeInstancesAndFiles(f []*WorkshopFile, instances []*Workshop) ([]*WorkshopFile, []*Workshop) {
	files := make([]*WorkshopFile, len(f))
	copy(files, f)
	// Walk both lists from to build a list of workshops with their states
	for _, ws := range instances {
		finder := func(p *WorkshopFile) bool { return p.Name == ws.Name }
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

func (s *LxdBackend) RemoveWorkshop(ctx context.Context, name string) (err error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(ContextProjectId).(string)
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

func (s *LxdBackend) WorkshopFs(ctx context.Context, name string) (WorkshopFs, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
	}

	sftp, err := conn.GetInstanceFileSFTP(InstanceName(name, projectId))
	if err != nil {
		return nil, err
	}

	return NewWorkshopFs(sftp), nil
}

func (s *LxdBackend) CreateStateStorage(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	pid, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", ContextProjectId)
	}

	// Create the storage volume entry
	vol := api.StorageVolumesPost{}
	vol.Name = WorkshopStateVolumeName(name, pid)
	vol.Type = "custom"
	vol.ContentType = "filesystem"
	vol.Config = map[string]string{}

	return conn.CreateStoragePoolVolume("default", vol)
}

func (s *LxdBackend) DeleteStateStorage(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	pid, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", ContextProjectId)
	}

	return conn.DeleteStoragePoolVolume("default", "custom", WorkshopStateVolumeName(name, pid))
}

func (s *LxdBackend) LxdClient(ctx context.Context) (lxd.InstanceServer, error) {
	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return nil, fmt.Errorf("context key %s not found", ContextUser)
	}

	if srv, err := lxd.ConnectLXDUnixWithContext(ctx, LxdSock, nil); err != nil {
		return nil, err
	} else {
		if err = InitProject(srv, user); err != nil {
			return nil, err
		}
		return srv.UseProject(LxdProjectName(user)), nil
	}
}

func createDefaultDevices() map[string]map[string]string {
	return map[string]map[string]string{
		"root":                 {"type": "disk", "pool": "default", "path": "/"},
		"workshop.network":     {"type": "nic", "network": "lxdbr0", "name": "eth0"},
		"workshop.socket":      {"type": "disk", "source": dirs.SocketPath + ".untrusted", "path": filepath.Join(dirs.WorkshopBaseDir, ".workshop.socket.untrusted")},
		"workshop.workshopctl": {"type": "disk", "source": filepath.Join(dirs.ExecDir, "workshopctl"), "path": "/usr/bin/workshopctl"},
	}
}

func (s *LxdBackend) workshopConfig(projectId string, userid, groupid string, file *WorkshopFile) (map[string]string, error) {
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

func (s *LxdBackend) fetchRemoteImage(base string) (lxd.ImageServer, *api.Image, error) {
	var image *api.Image

	imageServer, err := ConnectSimpleStreams(imageServer, nil)
	if err != nil {
		return nil, nil, err
	}

	names := strings.Split(base, "@")
	if len(names) <= 1 {
		return nil, nil, fmt.Errorf("cannot find a base image for the workshop")
	}

	alias, _, err := imageServer.GetImageAlias(fmt.Sprintf("%s/amd64", names[1]))
	if err != nil {
		return nil, nil, err
	}

	image, _, err = imageServer.GetImage(alias.Target)
	if err != nil {
		return nil, nil, err
	}

	return imageServer, image, nil
}
