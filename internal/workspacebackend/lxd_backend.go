package workspacebackend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/canonical/workspace/internal/logger"
	"github.com/canonical/workspace/internal/osutil"
	"github.com/gorilla/websocket"
	"github.com/spf13/afero"
	"golang.org/x/exp/slices"

	lxd "github.com/lxc/lxd/client"

	"github.com/lxc/lxd/shared/api"
)

type ExecArgs struct {
	Command     []string
	UserId      int
	GroupId     int
	WorkDir     string
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
	*ExecControls
}

type ExecContext struct {
	Environment          map[string]string
	DescriptorWebsockets map[string]string
	WaitExecution        func(ctx context.Context) error
}

type LxdBackend struct {
}

const (
	lxdSock = "/var/snap/lxd/common/lxd/unix.socket"
	// path used in workspace to mount the project directory
	WorkspaceProjectPath = "/project"
	// name prefix for the workspaces that were made unavailable
	StashNamePrefix = "stash-"
)

var (
	ConnectSimpleStreams = lxd.ConnectSimpleStreams
	UserLookup           = user.Lookup
	NewProjectId         = allocateProjectId

	ErrWorkspaceNotFound = errors.New("workspace not found")
)

func New() WorkspaceBackend {
	server := LxdBackend{}
	return &server
}

func LxdProjectName(user string) string {
	return "workspace." + user
}

func LxdSystemProjectName(user string) string {
	return LxdProjectName(user) + ".system"
}

func allocateProjectId() (string, error) {
	bytes := make([]byte, 4)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *LxdBackend) loadProject(client lxd.InstanceServer, ctx context.Context, path string) (*Project, error) {

	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return nil, fmt.Errorf("context key %s not found", ContextUser)
	}

	pId, err := projectId(LockPath(path))
	lockNotFound := false
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	} else if errors.Is(err, os.ErrNotExist) {
		lockNotFound = true
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

	// did we find a .workspace.lock in the path?
	if lockNotFound {
		// try to recover .workspace.lock file for this project
		// if it existed before and was accidentally removed
		for _, i := range projects {
			if i.Path == path {
				// save the lock file in the project's location
				i.UpdateLockFile()
				return i, nil
			}
		}
	} else {
		if val, ok := projects[pId]; ok {
			return val, nil
		}
	}
	return nil, ErrProjectNotFound
}

func (s *LxdBackend) untrackProject(client lxd.InstanceServer, ctx context.Context, prj *Project) error {
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

	delete(projects, prj.ProjectId)

	buf, err := json.Marshal(projects)
	if err != nil {
		return err
	}
	lxdPrj.ProjectPut.Config["user.workspace.projects"] = string(buf)

	return client.UpdateProject(LxdProjectName(user), lxdPrj.ProjectPut, etag)
}

func (s *LxdBackend) trackProject(client lxd.InstanceServer, ctx context.Context, prj *Project) error {
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

	return client.UpdateProject(LxdProjectName(user), lxdPrj.ProjectPut, etag)
}

// projectPath returns a project path for the cwd provided
// if cwd is a sub-directory of the project. Otherwise, cwd
// is returned unchanged
func ProjectPath(cwd string) (string, error) {
	path, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", nil
	}

	for {
		var err error
		var ok, isDir bool
		if ok, isDir, err = osutil.ExistsIsDir(path); err == nil && ok && isDir {
			if ok = osutil.FileExists(LockPath(path)); ok {
				return filepath.Clean(path), nil
			}
		}
		if err != nil {
			return "", err
		}

		if path == string(os.PathSeparator) {
			break
		}
		path = filepath.Join(path, "..", string(os.PathSeparator))
	}

	return cwd, nil
}

func (s *LxdBackend) CreateOrLoadProject(ctx context.Context, path string) (*Project, bool, error) {
	var err error

	if !filepath.IsAbs(path) {
		return nil, false, ErrNoRelativePathsAllowed
	}

	projectDir, err := ProjectPath(path)
	if err != nil {
		return nil, false, err
	}

	client, err := s.LxdClient(ctx)
	if err != nil {
		return nil, false, err
	}

	// see if we have this project already existing
	if existingProject, err := s.loadProject(client, ctx, projectDir); err == nil {
		// the tracked path and the requested path must be the same
		// otherwise it means that the project directory was moved or copied
		// If that is the case, we must update the project's configuration
		// in the LXD user.* key (i.e. track the project path with the existing id)
		if existingProject.Path != projectDir {
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
				existingProject.Path = projectDir
				err := s.trackProject(client, ctx, existingProject)
				if err != nil {
					return nil, false, err
				}
				// also, update configuration of all the project's workspaces
				projectCtx := context.WithValue(ctx, ContextProjectId, existingProject.ProjectId)
				return existingProject, false, s.updateWorkspacesProjectPath(client, projectCtx, existingProject)
			} else {
				// the directory was copied, so we:
				// 1. Generate a new project id for the actual path and update .lock file
				// 2. Start tracking the actual path as a new project
				id, err := NewProjectId()
				if err != nil {
					return nil, false, err
				}
				var newPrj = Project{Path: projectDir, ProjectId: id}

				if err = newPrj.UpdateLockFile(); err != nil {
					return nil, false, err
				}
				s.trackProject(client, ctx, &newPrj)
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
	var project = Project{Path: projectDir}
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
	if err = s.trackProject(client, ctx, &project); err != nil {
		return nil, false, err
	}

	return &project, true, nil
}

func (s *LxdBackend) updateWorkspacesProjectPath(conn lxd.InstanceServer, ctx context.Context, existingProject *Project) error {
	workspaces, err := s.filterLxdInstancesByConfig(conn, NewWorkspaceConfigFilter(ProjectIdConfig, existingProject.ProjectId))
	if err != nil {
		return err
	}

	for _, i := range workspaces {
		err = s.AddWorkspaceDevice(ctx, WorkspaceName(i.Name), WorkspaceDevice{
			Name:       ProjectPathDevice,
			Properties: map[string]string{"type": "disk", "source": existingProject.Path, "path": WorkspaceProjectPath},
		})
		if err != nil {
			return fmt.Errorf("cannot update workspace \"%v\" project directory", i.Name)
		}
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

	for _, prj := range projects {
		if ok, _, err := osutil.ExistsIsDir(prj.Path); !ok && err == nil {
			// If got here then there is no project directory for the projectId
			// anymore. It can mean moving or deletion happened in the past. Try
			// to recover the new project path
			newPath, _ := s.findProjectPathFromBindMounts(client, ctx, prj)
			if newPath != "" {
				// start tracking this project under a new path
				prj.Path = newPath
				if err = s.trackProject(client, ctx, prj); err == nil {
					// update the workspaces configuration with the new path
					s.updateWorkspacesProjectPath(client, ctx, prj)
				}
				continue
			}
			// Could not recover the directory, reconcile the project from the
			// list of projects that we track (only if there are no remaining
			// workspaces for this project)
			inst, err := s.filterLxdInstancesByConfig(client, func(config map[string]string) bool {
				return config["user.workspace.project-id"] == prj.ProjectId
			})
			if err == nil && len(inst) == 0 {
				if err = s.untrackProject(client, ctx, prj); err != nil {
					return nil, err
				}
				delete(projects, prj.ProjectId)
			}
		}
	}

	return projects, nil
}

func (s *LxdBackend) findProjectPathFromBindMounts(conn lxd.InstanceServer, ctx context.Context, p *Project) (path string, err error) {
	workspaces, err := s.filterLxdInstancesByConfig(conn, NewWorkspaceConfigFilter(ProjectIdConfig, p.ProjectId))
	if err != nil {
		return "", err
	}

	/* memFs to story temporary results of the commands execution output */
	memFs := afero.NewMemMapFs()
	for _, i := range workspaces {
		// attempt to execute the command only in a running instance
		if i.StatusCode != api.Ready && i.StatusCode != api.Running {
			continue
		}

		/* Take the first instance from the group, we need any running
		and ready to execute commands to validate the project directory */
		out, err := memFs.Create(i.Name)
		if err != nil {
			return "", err
		}
		defer out.Close()

		/* Get the mount point device/directory from findmnt and extract the path without a device
		using awk */
		args := Execution{
			ExecArgs: ExecArgs{
				UserId:  0,
				GroupId: 0,
				Command: []string{"bash", "-c",
					"findmnt --mountpoint /project -o source -n | awk -F\"[][]\" '{printf $2}'"},
				WorkDir: "/",
			},
			ExecControls: &ExecControls{
				Stdin:  nil,
				Stdout: out,
				Stderr: out,
			},
		}

		execCtx := context.WithValue(ctx, ContextProjectId, p.ProjectId)
		meta, err := s.execCommand(conn, execCtx, WorkspaceName(i.Name), &args)
		if err == nil {
			err = meta.WaitExecution(ctx)
			if err != nil {
				outbuf, _ := afero.ReadFile(memFs, out.Name())
				logger.Debugf("cannot check %q bind-mounts: %v, findmnt output: %s", i.Name, err, string(outbuf))
				continue
			}
		} else {
			logger.Debugf("cannot check %q bind-mounts: %v", i.Name, err)
			continue
		}

		/* Process the findmnt results */
		if currentPath, err := afero.ReadFile(memFs, i.Name); err == nil {
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

	userName, ok := ctx.Value(ContextUser).(string)
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
	}

	usr, err := UserLookup(userName)
	if err != nil {
		return err
	}

	req := api.InstancesPost{
		InstancePut: api.InstancePut{
			Devices: defaultDevices(),
			Config:  defaultConfig(projectId, usr.Uid, usr.Gid),
		},
		Name: InstanceName(name, projectId),
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

	_, _, err = conn.GetImageAlias(base)
	if err != nil && !api.StatusErrorCheck(err, http.StatusNotFound) {
		return err
	} else if api.StatusErrorCheck(err, http.StatusNotFound) {
		if err = conn.CreateImageAlias(api.ImageAliasesPost{ImageAliasesEntry: api.ImageAliasesEntry{
			Name: base,
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

	/* Do nothing if the instance is already in the desired state */
	if (inst.StatusCode == api.Running && action == "start") ||
		(inst.StatusCode == api.Stopped && action == "stop") {
		return nil
	}

	req := api.InstanceStatePut{
		Action:  action,
		Timeout: 30,
		Force:   force,
	}

	op, err := conn.UpdateInstanceState(inst.Name, req, etag)
	if err != nil {
		return err
	}

	return op.WaitContext(ctx)
}

func (s *LxdBackend) StartWorkspace(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
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
		ExecControls: &ExecControls{
			Stdin:  nil,
			Stdout: nil,
			Stderr: nil,
		},
	}

	exectx, err := s.execCommand(conn, ctx, name, &args)
	if err != nil {
		return err
	}

	return exectx.WaitExecution(ctx)
}

func (s *LxdBackend) StopWorkspace(ctx context.Context, name string, force bool) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	return s.updateInstanceState(conn, ctx, name, "stop", force)
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
	op, err := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)
	if err != nil {
		return err
	}

	return op.WaitContext(ctx)
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

func (s *LxdBackend) execCommand(conn lxd.InstanceServer, ctx context.Context, name string, args *Execution) (ExecContext, error) {
	websocket := func(opId, secret string) string {
		return fmt.Sprintf("ws://lxd.unix/1.0/operations/%s/websocket?secret=%s", opId, secret)
	}

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

	if args.ExecControls == nil {
		// create an execution object for the client to connect to
		op, err := conn.ExecInstance(InstanceName(name, projectId), req, nil)
		if err != nil {
			return ExecContext{}, err
		}

		opmeta := op.Get()
		var env = map[string]string{}
		var fds = map[string]string{}

		for k, v := range opmeta.Metadata["environment"].(map[string]any) {
			if value, ok := v.(string); ok {
				env[k] = value
			}
		}

		for k, v := range opmeta.Metadata["fds"].(map[string]any) {
			if value, ok := v.(string); ok {
				fds[k] = value
			}
		}

		return ExecContext{
			Environment: env,
			DescriptorWebsockets: map[string]string{
				"stdio":   websocket(opmeta.ID, fds["0"]),
				"stdout":  websocket(opmeta.ID, fds["1"]),
				"stderr":  websocket(opmeta.ID, fds["2"]),
				"control": websocket(opmeta.ID, fds["control"]),
			},
			WaitExecution: func(ctx context.Context) error {
				err := op.WaitContext(ctx)
				if err != nil {
					return err
				}
				var status = int(op.Get().Metadata["return"].(float64))
				if status != 0 {
					return &ErrExec{Status: status}
				}
				return nil
			},
		}, nil
	} else {
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
			Environment:          env,
			DescriptorWebsockets: map[string]string{},
			WaitExecution: func(ctx context.Context) error {
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
}

func (s *LxdBackend) Exec(ctx context.Context, name string, args *Execution) (ExecContext, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return ExecContext{}, err
	}

	return s.execCommand(conn, ctx, name, args)
}

func (s *LxdBackend) GetWorkspace(ctx context.Context, name string) (*Workspace, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
	}

	var p *Project
	projects, err := s.Projects(ctx)
	if err != nil {
		return nil, err
	}

	if p, ok = projects[projectId]; !ok {
		return nil, fmt.Errorf("project is not available: %v", projectId)
	}

	inst, _, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return nil, ErrWorkspaceNotFound
		}
		return nil, err
	}

	workspace, err := s.loadWorkspace(inst, p)
	if err != nil {
		return nil, err
	}

	return workspace, nil
}

func (s *LxdBackend) loadWorkspace(inst *api.Instance, p *Project) (*Workspace, error) {
	var err error
	var running, ok bool
	var pId string

	name := WorkspaceName(inst.Name)

	if pId, ok = inst.Config["user.workspace.project-id"]; !ok {
		return nil, fmt.Errorf("no project assossiated with the workspace %q", name)
	}

	if inst.StatusCode == api.Running || inst.StatusCode == api.Ready {
		running = true
	}

	base := inst.Config["image.os"] + "@" + inst.Config["image.version"]
	if base == "" {
		base = "unknown"
	}

	var workspace = &Workspace{
		backend:   s,
		projectId: pId,
		Name:      name,
		running:   running,
		base:      base,
	}

	workspace.Devices = inst.Devices

	file, err := p.WorkspaceFile(name)
	if err != nil {
		workspace.AddError(MissingFile)
	}
	workspace.SetFile(file)

	if exists, isDir, _ := osutil.ExistsIsDir(p.Path); !exists || !isDir {
		workspace.AddError(MissingProject)
	}

	// Fetch information about the installed SDKs
	workspace.content, err = InstalledContent(inst.Config)
	if err != nil {
		return nil, fmt.Errorf("cannot load workspace: installed SDK content is not readable: %v", err)
	}
	return workspace, nil
}

func (s *LxdBackend) filterLxdInstancesByConfig(conn lxd.InstanceServer, filter WorkspaceConfigFilter) ([]api.Instance, error) {
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

func (s *LxdBackend) GetProjectWorkspaces(ctx context.Context) ([]*WorkspaceFile, []*Workspace, error) {
	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, nil, fmt.Errorf("context key project-id not found")
	}

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, nil, err
	}

	var p *Project
	projects, err := s.Projects(ctx)
	if err != nil {
		return nil, nil, err
	}
	if p, ok = projects[projectId]; !ok {
		return nil, nil, fmt.Errorf("project is not available: %v", projectId)
	}

	files, err := p.EnumWorkspaceFiles()
	// if the dir does not exist it does not mean there are no workspaces. It
	// could be because the dir was removed with some workspaces still operating
	// resulting in a missing-project error
	if err != nil && !osutil.IsDirNotExist(err) {
		return nil, nil, err
	}

	// get all the running workspaces for this project
	instances, err := conn.GetInstances(api.InstanceTypeContainer)
	if err != nil {
		return nil, nil, err
	}

	var projectWorkspaces []*Workspace
	for _, i := range instances {
		if i.Config[ProjectIdConfig] == p.ProjectId {
			ws, err := s.loadWorkspace(&i, p)
			if err != nil {
				logger.Debugf("error loading workspace: %v", err)
				continue
			}
			projectWorkspaces = append(projectWorkspaces, ws)
		}
	}

	wsFiles, wsInstances := mergeInstancesAndFiles(files, projectWorkspaces)
	return wsFiles, wsInstances, nil
}

// Examine the lists of project's workspace files and workspaces. Returns two
// lists. The first has *only* the workspace files that do not have any launched
// workspaces yet, the second contains workspaces that are launched with or
// without an associated file.
func mergeInstancesAndFiles(f []*WorkspaceFile, instances []*Workspace) ([]*WorkspaceFile, []*Workspace) {
	files := make([]*WorkspaceFile, len(f))
	copy(files, f)
	/* Walk both lists from to build a list of workspaces with their states */
	for _, ws := range instances {
		finder := func(p *WorkspaceFile) bool { return p.Name == ws.Name }
		idx := slices.IndexFunc(files, finder)
		if idx != -1 {
			/* Both a file and instance exist */
			files = slices.Delete(files, idx, idx+1)
		}
	}

	/* At this point, files contain only inactive workspaces and instances
	contain the workspaces that have workspace files available */
	return files, instances
}

func (s *LxdBackend) RemoveWorkspace(ctx context.Context, name string) error {
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
		if err = s.updateInstanceState(conn, ctx, name, "stop", true); err != nil {
			return err
		}
	}

	op, err := conn.DeleteInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	return op.WaitContext(ctx)
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

func (s *LxdBackend) RemoveWorkspaceStash(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", ContextUser)
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	system := conn.UseProject(LxdSystemProjectName(user))
	op, err := system.DeleteInstance(InstanceName(StashNamePrefix+name, projectId))
	if err != nil {
		return err
	}
	return op.WaitContext(ctx)
}

func (s *LxdBackend) UnstashWorkspace(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	if err := s.moveInstanceProject(conn, ctx, name, false); err != nil {
		return err
	}

	if err := s.updateInstanceState(conn, ctx, name, "start", false); err != nil {
		return err
	}
	return nil
}

func (s *LxdBackend) StashWorkspace(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	if err := s.updateInstanceState(conn, ctx, name, "stop", false); err != nil {
		return err
	}

	return s.moveInstanceProject(conn, ctx, name, true)
}

// Moves the instance between projects. If system is true the project will be
// moved to the LXD project which is not available to the users (e.g. for
// hiding a workspace temporarily). Otherwise, the workspace will be move to
// the regular project visible by the user specified in the ctx context.
func (s *LxdBackend) moveInstanceProject(conn lxd.InstanceServer, ctx context.Context, name string, system bool) error {
	var err error
	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", ContextUser)
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	var op lxd.Operation
	instance := InstanceName(name, projectId)
	if system {
		// the new name must not be the same, otherwise the LXD's DNS will fail
		// the new instance creation
		op, err = conn.MigrateInstance(instance, api.InstancePost{
			Name:      StashNamePrefix + instance,
			Project:   LxdSystemProjectName(user),
			Migration: true,
		})
	} else {
		// the instance of interest is in the system project now, so let's
		// switch the project for the connection first to make the reverse
		// migration successful (i.e. from system -> user project)
		conn = conn.UseProject(LxdSystemProjectName(user))
		op, err = conn.MigrateInstance(StashNamePrefix+instance, api.InstancePost{
			Name:      instance,
			Project:   LxdProjectName(user),
			Migration: true,
		})
	}

	if err != nil {
		return err
	}

	err = op.WaitContext(ctx)
	return err
}

func (s *LxdBackend) CreateStateStorage(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

	pid, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", ContextProjectId)
	}

	// Create the storage volume entry
	vol := api.StorageVolumesPost{}
	vol.Name = WorkspaceStateVolumeName(name, pid)
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

	pid, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", ContextProjectId)
	}

	return conn.DeleteStoragePoolVolume("default", "custom", WorkspaceStateVolumeName(name, pid))
}

func (s *LxdBackend) LxdClient(ctx context.Context) (lxd.InstanceServer, error) {
	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return nil, fmt.Errorf("context key %s not found", ContextUser)
	}

	if srv, err := lxd.ConnectLXDUnixWithContext(ctx, lxdSock, nil); err != nil {
		return nil, err
	} else {
		if err = InitProject(srv, user); err != nil {
			return nil, err
		}
		return srv.UseProject(LxdProjectName(user)), nil
	}
}

func defaultDevices() map[string]map[string]string {
	return map[string]map[string]string{
		"root":              {"type": "disk", "pool": "default", "path": "/"},
		"workspace.network": {"type": "nic", "network": "lxdbr0", "name": "eth0"},
	}
}

func defaultConfig(projectId string, userid, groupid string) map[string]string {
	return map[string]string{
		"raw.idmap":                 fmt.Sprint("uid ", userid, " 1000\ngid ", groupid, " 1000"),
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
