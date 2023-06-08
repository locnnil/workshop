package workspacebackend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/canonical/workspace/internal/logger"
	"github.com/canonical/workspace/internal/osutil"
	"github.com/spf13/afero"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	lxd "github.com/lxc/lxd/client"

	"github.com/lxc/lxd/shared/api"
)

type ExecArgs struct {
	User    string
	Command []string
	WorkDir string
	Stdin   io.ReadCloser
	Stdout  io.WriteCloser
	Stderr  io.WriteCloser
}

type LxdBackend struct {
}

const (
	LxdSock = "/var/snap/lxd/common/lxd/unix.socket"
)

var (
	ConnectSimpleStreams = lxd.ConnectSimpleStreams
	NewProjectId         = allocateProjectId
)

func New() WorkspaceBackend {
	server := LxdBackend{}
	return &server
}

func LxdProjectName(user string) string {
	return "workspace." + user
}

func allocateProjectId() (string, error) {
	bytes := make([]byte, 4)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *LxdBackend) getProject(ctx context.Context, path string) (*Project, error) {
	client, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}

	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return nil, fmt.Errorf("context key %s not found", ContextUser)
	}

	pId, err := projectId(LockPath(path))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	lxdPrj, _, err := client.GetProject(LxdProjectName(user))
	if err != nil {
		return nil, err
	}

	var projects map[string]*Project = make(map[string]*Project, 0)
	if buf, ok := lxdPrj.Config["user.workspace.projects"]; ok {
		if err = json.Unmarshal([]byte(buf), &projects); err != nil {
			return nil, err
		}
	}

	if val, ok := projects[pId]; ok {
		// the tracked path and the requested path must be the same
		// otherwise it means that the project directory was moved or copied
		// If that is the case, we must update the project's configuration
		return val, nil
	}
	return nil, ErrProjectNotFound
}

func (s *LxdBackend) trackProject(ctx context.Context, prj *Project) error {
	client, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", ContextUser)
	}

	lxdPrj, etag, err := client.GetProject(LxdProjectName(user))
	if err != nil {
		return err
	}

	var projects map[string]*Project = make(map[string]*Project, 0)
	if buf, ok := lxdPrj.Config["user.workspace.projects"]; ok {
		if err = json.Unmarshal([]byte(buf), &projects); err != nil {
			return err
		}
	}

	projects[prj.ProjectId] = prj

	buf, err := json.Marshal(projects)
	if err != nil {
		return err
	}
	lxdPrj.ProjectPut.Config["user.workspace.projects"] = string(buf)

	if err = client.UpdateProject(LxdProjectName(user), lxdPrj.ProjectPut, etag); err != nil {
		return err
	}
	return nil
}

func (s *LxdBackend) CreateOrLoadProject(ctx context.Context, path string) (*Project, bool, error) {
	var err error

	if !filepath.IsAbs(path) {
		return nil, false, ErrNoRelativePathsAllowed
	}

	actualPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, false, err
	}

	// see if we have this project already existing
	if existingProject, err := s.getProject(ctx, actualPath); err == nil {
		// the tracked path and the requested path must be the same
		// otherwise it means that the project directory was moved or copied
		// If that is the case, we must update the project's configuration
		// in the LXD user.* key (i.e. track the project path with the existing id)
		if existingProject.Path != actualPath {
			// Was the project directory moved or copied?
			_, err := os.Stat(existingProject.Path)
			copied := true
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, false, err
			} else if errors.Is(err, os.ErrNotExist) {
				copied = false
			}

			if !copied {
				// the directory was moved, so we:
				// 1. Update the new path to the actual and track it
				// 2. Update all the workspaces to the new project mount
				existingProject.Path = actualPath
				err := s.trackProject(ctx, existingProject)
				if err != nil {
					return nil, false, err
				}
				// also, update configuration of all the project's workspaces
				return existingProject, false, s.updateWorkspacesProjectPath(ctx, existingProject)
			} else {
				// the directory was copied, so we:
				// 1. Generate a new project id for the actual path and update .lock file
				// 2. Start tracking the actual path as a new project
				id, err := NewProjectId()
				if err != nil {
					return nil, false, err
				}
				var newPrj = Project{Path: actualPath, ProjectId: id}

				if err = newPrj.UpdateLockFile(); err != nil {
					return nil, false, err
				}
				s.trackProject(ctx, &newPrj)
				return &newPrj, true, nil
			}
		}
		return existingProject, false, nil
	} else if !errors.Is(err, ErrProjectNotFound) {
		// if there is some error that is unrelated to the
		// project loadOrCreate logic (e.g. failed to connect to LXD)
		// then return the error immediately
		return nil, false, err
	}

	// no project found, try to create one, note there is no ID yet at this stage
	var project = Project{Path: actualPath}
	workspaces, err := project.EnumWorkspaceFiles()
	if err != nil {
		return nil, false, err
	}

	// no workspaces found in the directory provided
	// it means we won't be creating a project
	if len(workspaces) == 0 {
		return nil, false, ErrNotAProject
	}

	// If there is at least one workspace, we consider the path
	// as a project and create a new project id
	if project.ProjectId, err = NewProjectId(); err != nil {
		return nil, false, err
	} else {
		// if we allocated a new project ID successfully,
		// we store it in the lock file immediately
		if err = project.UpdateLockFile(); err != nil {
			return nil, false, err
		}
	}

	// Now, add the project ID to the tracking map
	// stored in a custom user.* key of the LXD project for this user
	if err = s.trackProject(ctx, &project); err != nil {
		return nil, false, err
	}

	return &project, true, nil
}

func (s *LxdBackend) updateWorkspacesProjectPath(ctx context.Context, existingProject *Project) error {
	workspaces, err := s.GetWorkspacesByConfig(ctx, NewWorkspaceConfigFilter(ProjectIdConfig, existingProject.ProjectId))
	if err != nil {
		return err
	}

	for _, i := range workspaces {
		s.AddWorkspaceDevice(ctx, i.Name, WorkspaceDevice{
			Name:       ProjectPathDevice,
			Properties: map[string]string{"type": "disk", "source": existingProject.Path, "path": "/project"},
		})
	}
	return nil
}

func (s *LxdBackend) Projects(ctx context.Context) (map[string]*Project, error) {
	client, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}

	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return nil, fmt.Errorf("context key %s not found", ContextUser)
	}

	lxdPrj, _, err := client.GetProject(LxdProjectName(user))
	if err != nil {
		return nil, err
	}

	var projects map[string]*Project = make(map[string]*Project, 0)
	if buf, ok := lxdPrj.Config["user.workspace.projects"]; ok {
		if err = json.Unmarshal([]byte(buf), &projects); err != nil {
			return nil, err
		}
	}

	for _, i := range projects {
		if ok, _, err := osutil.ExistsIsDir(i.Path); !ok && err == nil {
			// we realised that there is no project directory for the
			// projectId anymore. It can mean moving or deletion happened in the past
			// Try to recover the new project path
			newPath, err := s.findProjectPathFromBindMounts(ctx, i)
			if err == nil && newPath != "" {
				// start tracking this project under a new path
				i.Path = newPath
				if err = s.trackProject(ctx, i); err == nil {
					s.updateWorkspacesProjectPath(ctx, i)
				}
			}
		}
	}

	return projects, nil
}

func (s *LxdBackend) findProjectPathFromBindMounts(ctx context.Context, p *Project) (path string, err error) {
	workspaces, err := s.GetWorkspacesByConfig(ctx, NewWorkspaceConfigFilter(ProjectIdConfig, p.ProjectId))
	if err != nil {
		return "", err
	}

	/* memFs to story temporary results of the commands execution output */
	memFs := afero.NewMemMapFs()
	for _, i := range workspaces {
		if i.state != Ready {
			continue
		}

		/* Take the first instance from the group, we need any running
		and ready to execute commands to validate the project directory */
		stdout, err := memFs.Create(InstanceName(i.Name, p.ProjectId))
		if err != nil {
			return "", err
		}

		/* Get the mount point device/directory from findmnt and extract the path without a device
		using awk */
		args := ExecArgs{User: "root",
			Command: []string{"bash", "-c",
				"findmnt --mountpoint /project -o source -n | awk -F\"[][]\" '{printf $2}'"},
			WorkDir: "/",
			Stdin:   nil,
			Stdout:  stdout,
			Stderr:  nil}

		execCtx := context.WithValue(ctx, ContextProjectId, p.ProjectId)
		done, err := s.Exec(execCtx, i.Name, &args)
		if err != nil {
			continue
		}
		<-done

		/* Process the findmnt results */
		if currentPath, err := afero.ReadFile(memFs, InstanceName(i.Name, p.ProjectId)); err == nil {
			/* check if the path is not //deleted, i.e. the project directory still exists on the host */
			if ok, isDir, err := osutil.ExistsIsDir(string(currentPath)); ok && isDir {
				return string(currentPath), nil
			} else if err != nil && !osutil.IsDirNotExist(err) {
				return "", nil
			}
		}
	}
	return "", nil
}

func (s *LxdBackend) LaunchWorkspace(ctx context.Context, name, base string) error {
	var err error
	var imageSrv lxd.ImageServer
	var image *api.Image

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key user not found")
	}

	/* Skip if the instance exists already */
	if _, _, err := conn.GetInstance(InstanceName(name, projectId)); err == nil {
		return fmt.Errorf("workspace \"%s\" already exists", name)
	}

	/* Check if we have the base image stored locally */
	if alias, _, err := conn.GetImageAlias(base); err == nil {
		if image, _, err = conn.GetImage(alias.Target); err != nil {
			return err
		}
		imageSrv = conn
	} else {
		imageSrv, image, err = s.fetchRemoteImage(base)
		if err != nil {
			return err
		}

		defer conn.CreateImageAlias(api.ImageAliasesPost{ImageAliasesEntry: api.ImageAliasesEntry{
			Name: base,
			ImageAliasesEntryPut: api.ImageAliasesEntryPut{
				Target: image.Fingerprint,
			},
		}})
	}

	req := api.InstancesPost{
		InstancePut: api.InstancePut{
			Devices: defaultDevices(),
			Config:  defaultConfig(projectId),
		},
		Name: InstanceName(name, projectId),
		Type: api.InstanceType("container"),
		Source: api.InstanceSource{
			Type:        "image",
			Fingerprint: image.Fingerprint,
			Project:     LxdProjectName(user),
		},
	}
	op, err := conn.CreateInstanceFromImage(imageSrv, *image, req)
	if err != nil {
		return err
	}

	return op.Wait()
}

func (s *LxdBackend) SetWorkspaceState(ctx context.Context, name, action string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, _, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	/* Do nothing if the instance is already in the desired state */
	if (inst.StatusCode == api.Running && action == "start") ||
		(inst.StatusCode == api.Stopped && action == "stop") {
		return nil
	}

	req := api.InstanceStatePut{
		Action:  action,
		Timeout: 5,
		Force:   false,
	}

	op, err := conn.UpdateInstanceState(inst.Name, req, "")
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}
	return err
}

func (s *LxdBackend) AddWorkspaceConfig(ctx context.Context, name string, item *WorkspaceConfigValue) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}
	inst.Config[item.Name] = item.Value
	op, _ := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdBackend) RemoveWorkspaceConfig(ctx context.Context, name string, key string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	delete(inst.Config, key)
	op, _ := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdBackend) AddWorkspaceDevice(ctx context.Context, name string, device WorkspaceDevice) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}
	inst.Devices[device.Name] = device.Properties
	op, _ := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdBackend) RemoveWorkspaceDevice(ctx context.Context, name string, device string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

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

func (s *LxdBackend) Exec(ctx context.Context, name string, args *ExecArgs) (chan bool, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
	}

	req := api.InstanceExecPost{
		Command: args.Command, WaitForWS: true,
		User: 0, Group: 0, Cwd: args.WorkDir,
		Interactive: false,
	}

	done := make(chan bool)

	arg := lxd.InstanceExecArgs{
		Stdin: args.Stdin, Stdout: args.Stdout, Stderr: args.Stderr,
		Control: nil, DataDone: done,
	}

	op, err := conn.ExecInstance(InstanceName(name, projectId), req, &arg)

	if err != nil {
		return done, err
	}

	if err := op.WaitContext(ctx); err != nil {
		return done, err
	}

	if status, ok := op.Get().Metadata["return"].(float64); ok {
		if status != 0 {
			logger.Debugf("command execution failed with %v", int(status))
			return done, &ErrExec{Status: int(status)}
		}
	}

	return done, nil
}

func (s *LxdBackend) GetWorkspace(ctx context.Context, name string) (*WorkspaceProps, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
	}

	inst, _, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return nil, err
	}
	return &WorkspaceProps{
		Name:    name,
		state:   fromLxdToWorkspaceState(inst.StatusCode),
		Devices: inst.Devices,
		Config:  inst.Config,
	}, nil
}

func (s *LxdBackend) GetWorkspacesByConfig(ctx context.Context, filter WorkspaceConfigFilter) ([]*WorkspaceProps, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}

	instances, err := conn.GetInstances(api.InstanceTypeContainer)
	if err != nil {
		return nil, err
	}
	var ws []*WorkspaceProps = make([]*WorkspaceProps, 0, len(instances))
	for _, i := range instances {
		if filter(i.Config) {
			ws = append(ws, &WorkspaceProps{
				Name:    WorkspaceName(i.Name),
				state:   fromLxdToWorkspaceState(i.StatusCode),
				Devices: i.Devices,
				Config:  i.Config,
			})
		}
	}

	return ws, nil
}

func (s *LxdBackend) GetAllWorkspaces(ctx context.Context) ([]*WorkspaceProps, error) {
	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
	}

	// get the files for this project
	var p *Project
	projects, err := s.Projects(ctx)
	if err != nil {
		return nil, err
	}
	if p, ok = projects[projectId]; !ok {
		return nil, fmt.Errorf("project is not available: %v", projectId)
	}

	files, err := p.EnumWorkspaceFiles()
	if err != nil && !osutil.IsDirNotExist(err) {
		return nil, err
	}

	// get all the running workspaces for this project
	workspaces, err := s.GetWorkspacesByConfig(ctx, NewWorkspaceConfigFilter(ProjectIdConfig, p.ProjectId))
	if err != nil {
		return nil, err
	}

	// now merge the files and workspaces to have correct notes and statuses in the output
	return MergeInstancesAndFiles(files, workspaces), nil
}

func MergeInstancesAndFiles(f []*WorkspaceProps, i []*WorkspaceProps) []*WorkspaceProps {
	files, instances := make([]*WorkspaceProps, len(f)), make([]*WorkspaceProps, len(i))
	copy(files, f)
	copy(instances, i)
	/* Merge both lists from to build a list of workspaces with their states */
	result := make([]*WorkspaceProps, 0, len(files)+len(instances))
	for _, ws := range instances {
		finder := func(p *WorkspaceProps) bool { return p.Name == ws.Name }
		idx := slices.IndexFunc(files, finder)
		if idx == -1 {
			/* We only have an instance, no file (perhaps, there is no project directory)
			 */
			projectPath := ws.Devices[ProjectPathDevice]["source"]

			if exists, isDir, _ := osutil.ExistsIsDir(projectPath); exists && isDir {
				ws.SetState(Error, MissingFile)
			} else {
				ws.SetState(Error, MissingProject)
			}
		} else {
			/* Both a file and instance exist */
			files = slices.Delete(files, idx, idx+1)
		}
		result = append(result, ws)
	}

	/* Now, files contains only inactive workspaces */
	for _, ws := range files {
		ws.SetState(Off, None)
		result = append(result, ws)
	}
	return result
}

func (s *LxdBackend) GetWorkspacesByDevices(ctx context.Context, filter WorkspaceDeviceFilter) (map[string]*WorkspaceProps, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}

	instances, err := conn.GetInstances(api.InstanceTypeContainer)
	if err != nil {
		return nil, err
	}
	var ws map[string]*WorkspaceProps = make(map[string]*WorkspaceProps, len(instances))
	for _, i := range instances {
		if filter(i.Devices) {
			name := WorkspaceName(i.Name)
			ws[name] = &WorkspaceProps{
				Name:    name,
				state:   fromLxdToWorkspaceState(i.StatusCode),
				Devices: i.Devices,
				Config:  i.Config,
			}

		}
	}

	return ws, nil
}

func (s *LxdBackend) DeleteWorkspace(ctx context.Context, name string, forceful bool) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, _, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	if inst.StatusCode != 0 && inst.StatusCode != api.Stopped {
		if forceful {
			req := api.InstanceStatePut{
				Action:  "stop",
				Timeout: -1,
				Force:   true,
			}

			op, err := conn.UpdateInstanceState(inst.Name, req, "")
			if err != nil {
				return err
			}

			err = op.Wait()
			if err != nil {
				return fmt.Errorf("stopping the instance failed: %s", err)
			}
		} else {
			return fmt.Errorf("cannot delete a non-stopped workspace: %q", name)
		}
	}

	op, err := conn.DeleteInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	return op.Wait()
}

func (s *LxdBackend) GetWorkspaceFs(ctx context.Context, name string) (WorkspaceFs, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
	}

	sftp, err := conn.GetInstanceFileSFTP(InstanceName(name, projectId))
	if err != nil {
		return nil, err
	}

	return NewWorkspaceFs(sftp), nil
}

func (s *LxdBackend) LxdClient(ctx context.Context) (lxd.InstanceServer, error) {
	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return nil, fmt.Errorf("context key %s not found", ContextUser)
	}

	if srv, err := lxd.ConnectLXDUnixWithContext(ctx, LxdSock, nil); err != nil {
		return nil, err
	} else {
		if err = InitProject(srv, LxdProjectName(user)); err != nil {
			return nil, err
		}
		return srv.UseProject(LxdProjectName(user)), nil
	}
}

func fromLxdToWorkspaceState(lxdStatus api.StatusCode) WorkspaceState {
	var state WorkspaceState
	switch lxdStatus {
	case api.Running, api.Ready:
		state = Ready
	case api.Stopped:
		state = Stopped
	default:
		state = Pending
	}
	return state
}

func defaultDevices() map[string]map[string]string {
	return map[string]map[string]string{
		"root":              {"type": "disk", "pool": "default", "path": "/"},
		"workspace.network": {"type": "nic", "network": "lxdbr0", "name": "eth0"},
	}
}

func defaultConfig(projectId string) map[string]string {
	return map[string]string{
		"raw.idmap":                 fmt.Sprint("uid ", os.Getuid(), " 1000\ngid ", os.Getgid(), " 1000"),
		"security.nesting":          "true",
		"user.workspace.project-id": projectId,
	}
}

func (s *LxdBackend) fetchRemoteImage(base string) (lxd.ImageServer, *api.Image, error) {
	var image *api.Image

	imageServer, err := ConnectSimpleStreams("https://cloud-images.ubuntu.com/releases/", nil)
	if err != nil {
		return nil, nil, err
	}

	names := strings.Split(base, "@")
	if len(names) <= 1 {
		return nil, nil, fmt.Errorf("cannot find a base image for the workspace")
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

/* Fake backend implementation for tests */

type ExecFunc func(ctx context.Context, name string, args *ExecArgs) (chan bool, error)

type FakeWorkspaceBackend struct {
	Workspaces map[string]map[string]*WorkspaceProps
	projects   map[string]map[string]*Project
	WsFs       WorkspaceFs
	LocalFs    afero.Fs

	DoExec ExecFunc
}

func NewFakeWorkspaceBackend() *FakeWorkspaceBackend {
	var be FakeWorkspaceBackend
	be.Workspaces = make(map[string]map[string]*WorkspaceProps)
	be.projects = make(map[string]map[string]*Project)
	be.WsFs = NewFakeWorkspaceFs()
	be.LocalFs = afero.NewMemMapFs()

	be.DoExec = DoExecDefault

	return &be
}

func (s *FakeWorkspaceBackend) CreateOrLoadProject(ctx context.Context, path string) (*Project, bool, error) {
	username := ctx.Value(ContextUser).(string)
	if val, ok := s.projects[username]; ok {
		if prj, ok := val[path]; ok {
			return prj, false, nil
		}
	} else {
		s.projects[username] = make(map[string]*Project)
	}

	prjId, _ := NewProjectId()
	newPrj := &Project{ProjectId: prjId, Path: path}
	s.projects[username][path] = newPrj
	return newPrj, true, nil
}

func (s *FakeWorkspaceBackend) Projects(ctx context.Context) (map[string]*Project, error) {
	username := ctx.Value(ContextUser).(string)
	return s.projects[username], nil
}

func (f *FakeWorkspaceBackend) LaunchWorkspace(ctx context.Context, name, base string) error {
	projectId := ctx.Value(ContextProjectId).(string)
	if f.Workspaces[projectId] == nil {
		f.Workspaces[projectId] = make(map[string]*WorkspaceProps)
	}
	f.Workspaces[projectId][name] = &WorkspaceProps{
		Name:    name,
		Devices: defaultDevices(),
		Config:  defaultConfig(projectId),
		state:   Ready,
	}
	return nil
}

func (f *FakeWorkspaceBackend) DeleteWorkspace(ctx context.Context, name string, forceful bool) error {
	panic("not implemented") // TODO: Implement
}

func (f *FakeWorkspaceBackend) SetWorkspaceState(ctx context.Context, name, action string) error {
	panic("not implemented") // TODO: Implement
}

func (f *FakeWorkspaceBackend) AddWorkspaceDevice(ctx context.Context, name string, props WorkspaceDevice) error {
	projectId := ctx.Value(ContextProjectId).(string)
	f.Workspaces[projectId][name].Devices[props.Name] = props.Properties
	return nil
}

func (f *FakeWorkspaceBackend) RemoveWorkspaceDevice(ctx context.Context, name string, device string) error {
	projectId := ctx.Value(ContextProjectId).(string)
	delete(f.Workspaces[projectId][name].Devices, device)
	return nil
}

func (f *FakeWorkspaceBackend) AddWorkspaceConfig(ctx context.Context, name string, item *WorkspaceConfigValue) error {
	projectId := ctx.Value(ContextProjectId).(string)
	f.Workspaces[projectId][name].Config[item.Name] = item.Value
	return nil
}

func (f *FakeWorkspaceBackend) RemoveWorkspaceConfig(ctx context.Context, name string, key string) error {
	projectId := ctx.Value(ContextProjectId).(string)
	delete(f.Workspaces[projectId][name].Config, key)
	return nil
}

func (f *FakeWorkspaceBackend) GetWorkspace(ctx context.Context, name string) (*WorkspaceProps, error) {
	projectId := ctx.Value(ContextProjectId).(string)
	return f.Workspaces[projectId][name], nil
}

func (f *FakeWorkspaceBackend) GetAllWorkspaces(ctx context.Context) ([]*WorkspaceProps, error) {
	projectId := ctx.Value(ContextProjectId).(string)
	return maps.Values(f.Workspaces[projectId]), nil
}

func (f *FakeWorkspaceBackend) GetWorkspacesByConfig(ctx context.Context, filter WorkspaceConfigFilter) ([]*WorkspaceProps, error) {
	res := make([]*WorkspaceProps, 0)
	for _, i := range f.Workspaces {
		for _, j := range i {
			if filter(j.Config) {
				res = append(res, j)
			}
		}
	}
	return res, nil
}

func (f *FakeWorkspaceBackend) GetWorkspacesByDevices(ctx context.Context, filter WorkspaceDeviceFilter) (map[string]*WorkspaceProps, error) {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkspaceBackend) GetWorkspaceFs(ctx context.Context, name string) (WorkspaceFs, error) {
	return s.WsFs, nil
}

func (f *FakeWorkspaceBackend) Exec(ctx context.Context, name string, args *ExecArgs) (chan bool, error) {
	return f.DoExec(ctx, name, args)
}

func DoExecDefault(ctx context.Context, name string, args *ExecArgs) (chan bool, error) {
	done := make(chan bool)
	close(done)
	return done, nil
}
