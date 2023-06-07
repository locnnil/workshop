package project

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"regexp"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/canonical/workspace/internal/workspacebackend"
	backend "github.com/canonical/workspace/internal/workspacebackend"

	"github.com/spf13/afero"
)

type Project struct {
	Path      string `json:"path"`
	ProjectId string `json:"id"`

	fs afero.Fs
}

var (
	ErrProjectNotFound        = errors.New("project not found")
	ErrNoRelativePathsAllowed = errors.New("absolute project path must be used")
)

const (
	ProjectLock        = ".workspace.lock"
	ProjectDeviceField = "workspace.project"
	ProjectIdField     = "user.workspace.project-id"
)

// for testing purposes
var (
	RetrieveProject     = retrieveProject
	RetrieveAllProjects = retrieveAllProjects
	NewProject          = New
	EvalSymlinks        = filepath.EvalSymlinks
)

var validWorkspaceFilename = regexp.MustCompile(`^\.workspace\.(?P<name>[a-z_][a-z0-9_-]*)\.yaml$`)

func New(fs afero.Fs, path string) (*Project, error) {
	var err error
	var project Project

	/* Make sure the path is canonicalised */
	if path, err = CleanProjectPath(path); err != nil {
		return nil, err
	}

	project.fs = fs
	project.Path = path

	if projectId, err := allocateProjectId(); err != nil {
		return nil, err
	} else {
		if err = project.UpdateLockFile(projectId); err != nil {
			return nil, err
		}
	}

	return &project, nil
}

func retrieveProject(ctx context.Context, backend backend.WorkspaceBackend, fs afero.Fs, path string) (*Project, error) {
	var err error
	var project = Project{Path: path, fs: fs}

	/* Make sure the path is canonicalised */
	if path, err = CleanProjectPath(path); err != nil {
		return nil, err
	}

	/* Now let's find the project-id for the path */

	/* Is there an existing project file? */
	if buf, err := afero.ReadFile(fs, filepath.Join(path, ProjectLock)); err == nil {
		/* See if the project's directory was changed (thus, workspace config may be incorrect).
		We need to update its instances' configuration to maintain integrity */
		if err = project.validateWorkspaceConfiguration(ctx, backend); err != nil {
			return nil, err
		}
		project.ProjectId = string(buf)
	} else if errors.Is(err, afero.ErrFileNotFound) {
		/* no .lock file found, let's see if we ever had a lock file here */
		if id := recorverProjectId(ctx, backend, fs, path); id != "" {
			/* recovered project-id successfully, recreate .lock */
			if err = project.UpdateLockFile(id); err != nil {
				return nil, err
			}
		} else {
			/* There is no project in the given location, perhaps never was */
			return nil, fmt.Errorf("%v: %v", ErrProjectNotFound, path)
		}
	} else {
		return nil, fmt.Errorf("%v: %v", ErrProjectNotFound, path)
	}

	return &project, nil
}

func (p *Project) validateWorkspaceConfiguration(ctx context.Context, be backend.WorkspaceBackend) error {
	var err error

	/* list all the workspaces for this project */
	instances, err := be.GetWorkspacesByConfig(ctx, backend.NewWorkspaceConfigFilter(ProjectIdField, p.ProjectId))
	if err != nil {
		return err
	}

	/* see if any of the project's workspace has an incorrect config
	   to save on any unnecessary API calls to the server
	*/
	idx := slices.IndexFunc(instances, func(w *backend.WorkspaceProps) bool { return w.Devices[ProjectDeviceField]["source"] != p.Path })
	if idx == -1 {
		return nil
	}

	/* possibly, the original directory still exists due to, for example:
	   mv dir dir-1
	   workspace list --project dir-1
	   mv dir-1 dir                       <-- workspace will be blind to this
	   cp -R dir dir-copy
	   workspace list --project dir-copy

	   We should examine running workspaces (if any) to see if that's the case
	*/

	/* let's inspect all the workspaces' bind-mounts and update configs if required */
	if updated, err := updateConfigFromBindMounts(ctx, be, p.fs); err != nil {
		return err
	} else if updated {
		/* the workspaces' configuration was updated, so re-fetch the instances of the project */
		instances, err = be.GetWorkspacesByConfig(ctx, backend.NewWorkspaceConfigFilter(ProjectIdField, p.ProjectId))
		if err != nil {
			return err
		}
	}

	for _, i := range instances {
		source, deviceOk := i.Devices[ProjectDeviceField]["source"]
		/* if some of the workspaces do not have a correct configuration
		after running an update from bind-mounts, e.g. all of them were stopped */
		if source != p.Path {
			/* The directory was copied elsewhere, we need to generate a new project-id to let the old project-id persist */
			if ok, _ := afero.Exists(p.fs, LockPath(source)); ok && deviceOk {
				projectId, err := allocateProjectId()
				if err != nil {
					return err
				}
				return p.UpdateLockFile(projectId)
			}

			/* The directory was moved, or there is no project device for the workspace. Update the project mount */
			var mount = backend.WorkspaceDevice{
				Name:       ProjectDeviceField,
				Properties: map[string]string{"type": "disk", "source": p.Path, "path": "/project"},
			}
			prjCtx := context.WithValue(ctx, backend.ContextProjectId, p.ProjectId)
			be.AddWorkspaceDevice(prjCtx, i.Name, mount)
		}

	}
	return nil
}

func (w *Project) UpdateLockFile(projectId string) error {
	w.ProjectId = projectId
	return afero.WriteFile(w.fs, LockPath(w.Path), []byte(projectId), 0644)
}

func (w *Project) EnumWorkspaceFiles() ([]*backend.WorkspaceProps, error) {
	files, err := afero.ReadDir(w.fs, w.Path)
	if err != nil {
		return nil, err
	}

	var workspaces = make([]*backend.WorkspaceProps, 0, len(files))

	for _, info := range files {
		if info.IsDir() {
			continue
		}

		/* The first element in names will contain the workspace name if matched */
		if names := validWorkspaceFilename.FindStringSubmatch(info.Name()); names != nil {
			workspaces = append(workspaces, &backend.WorkspaceProps{Name: names[1]})
		}
	}
	return workspaces, nil
}

func (w *Project) RetrieveWorkspaces(ctx context.Context, be backend.WorkspaceBackend) ([]*backend.WorkspaceProps, error) {
	/* (1) Find all the project's workspace files */
	files, err := w.EnumWorkspaceFiles()
	// we handle the case when a project directory was removed, but there still could be
	// workspaces referring to it
	if err != nil && !errors.Is(err, afero.ErrFileNotFound) {
		return nil, err
	}

	/* (2) List all the project's instances */
	instances, err := be.GetWorkspacesByConfig(ctx, backend.NewWorkspaceConfigFilter(ProjectIdField, w.ProjectId))
	if err != nil {
		return nil, err
	}

	result := mergeInstancesAndFiles(w.fs, files, instances)

	return result, nil
}

/*
Check if the project DID exist in this directory path and if so, try to recover it.

	We generate .lock file in the projects directory which could be easily deleted by a user
	after running smth like make clean given .lock is supposed to not be tracked by a VCS. If such
	an accident happens the integrity of the previously created workspaces must not suffer. Here
	we attempt to recover a previously created .lock
*/
func recorverProjectId(ctx context.Context, be backend.WorkspaceBackend, fs afero.Fs, path string) string {
	// first, check all the workspaces to see if we can use
	// the existing bind mounts to update the workspaces' project configuration
	updateConfigFromBindMounts(ctx, be, fs)

	// now, when we have updated workspaces' configurations, let's see
	// if our path, and, thus, project-id can be found in the configuration
	instances, _ := be.GetWorkspacesByDevices(ctx, func(devices map[string]map[string]string) bool {
		if mount, ok := devices[ProjectDeviceField]; ok {
			if mount["source"] == path {
				return true
			}
		}
		return false
	})

	if len(instances) > 0 {
		/* Found a previous .lock file, restore it */
		for _, i := range instances {
			if id, ok := i.Config[ProjectIdField]; ok {
				return id
			}
		}
	}
	return ""
}

func mergeInstancesAndFiles(fs afero.Fs, files []*backend.WorkspaceProps, instances []*backend.WorkspaceProps) []*backend.WorkspaceProps {
	/* Merge both lists from to build a list of workspaces with their states */
	result := make([]*backend.WorkspaceProps, 0, len(files)+len(instances))
	for _, ws := range instances {
		finder := func(p *backend.WorkspaceProps) bool { return p.Name == ws.Name }
		idx := slices.IndexFunc(files, finder)
		if idx == -1 {
			/* We only have an instance, no file (perhaps, there is no project directory)
			 */
			projectPath := ws.Devices[ProjectDeviceField]["source"]
			if exists, _ := afero.DirExists(fs, projectPath); exists {
				ws.SetState(workspacebackend.Error, workspacebackend.MissingFile)
			} else {
				ws.SetState(workspacebackend.Error, workspacebackend.MissingProject)
			}
		} else {
			/* Both a file and instance exist */
			files = slices.Delete(files, idx, idx+1)
		}
		result = append(result, ws)
	}

	/* Now, files contains only inactive workspaces */
	for _, ws := range files {
		ws.SetState(workspacebackend.Off, workspacebackend.None)
		result = append(result, ws)
	}
	return result
}

func retrieveAllProjects(ctx context.Context, be backend.WorkspaceBackend, fs afero.Fs) ([]*Project, error) {
	updateConfigFromBindMounts(ctx, be, fs)
	all, err := be.GetWorkspacesByConfig(ctx, backend.EveryWorkspace())
	if err != nil {
		return nil, err
	}

	var projects = make(map[string]*Project, len(all))
	for _, i := range all {
		id := i.Config[ProjectIdField]
		if _, ok := projects[id]; !ok {
			path := i.Devices[ProjectDeviceField]["source"]
			prj, err := retrieveProject(ctx, be, fs, path)
			if err != nil && !errors.Is(err, afero.ErrFileNotFound) {
				continue
			} else if errors.Is(err, afero.ErrFileNotFound) {
				projects[id] = &Project{ProjectId: id, Path: path, fs: fs}
				continue
			}
			projects[id] = prj
		}
	}

	return maps.Values(projects), nil
}

func allocateProjectId() (string, error) {
	bytes := make([]byte, 4)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func LockPath(path string) string {
	return filepath.Join(path, ProjectLock)
}

/*
This function updates an instance's project mount in its config file by analysing its
actual bind-mounts and updating the configuration accordingly to avoid a situation when
a project directory listed in the instance configuration is different from the actual
bind mount because a directory was deleted, moved or copied.
*/
func updateConfigFromBindMounts(ctx context.Context, be backend.WorkspaceBackend, fs afero.Fs) (updated bool, err error) {
	workspaces, err := be.GetWorkspacesByConfig(ctx, backend.EveryWorkspace())
	if err != nil {
		return false, err
	}

	/* Group by instances by project-id and project directory  */
	type projectKey struct {
		path, id string
	}
	var grouped = make(map[projectKey][]*backend.WorkspaceProps, len(workspaces))
	for _, i := range workspaces {
		if i.State() == workspacebackend.Ready {
			projectPath := i.Devices[ProjectDeviceField]["source"]
			key := projectKey{projectPath, i.Config[ProjectIdField]}
			grouped[key] = append(grouped[key], i)
		}
	}

	/* memFs to story temporary results of the commands execution output */
	memFs := afero.NewMemMapFs()
	for key, i := range grouped {

		/* Take the first instance from the group, we need any running
		and ready to execute commands to validate the project directory */
		instance := i[0]
		stdout, err := memFs.Create(workspacebackend.InstanceName(instance.Name, key.id))
		if err != nil {
			return false, err
		}

		/* Get the mount point device/directory from findmnt and extract the path without a device
		using awk */
		args := backend.ExecArgs{User: "root",
			Command: []string{"bash", "-c",
				"findmnt --mountpoint /project -o source -n | awk -F\"[][]\" '{printf $2}'"},
			WorkDir: "/",
			Stdin:   nil,
			Stdout:  stdout,
			Stderr:  nil}

		prjCtx := context.WithValue(ctx, backend.ContextProjectId, key.id)
		done, err := be.Exec(prjCtx, instance.Name, &args)
		if err != nil {
			continue
		}
		<-done

		/* Process the findmnt results */
		if currentPath, err := afero.ReadFile(memFs, workspacebackend.InstanceName(instance.Name, key.id)); err == nil {
			/* check if the path is not //deleted, i.e. the project directory still exists on the host */
			if ok, _ := afero.Exists(fs, string(currentPath)); ok {
				if lxdPath, ok := instance.Devices[ProjectDeviceField]["source"]; ok {
					if lxdPath != string(currentPath) {
						/* now, update LXD configuration for all the group's instances */
						for _, inst := range i {
							be.AddWorkspaceDevice(prjCtx, inst.Name, backend.WorkspaceDevice{
								Name:       ProjectDeviceField,
								Properties: map[string]string{"type": "disk", "source": string(currentPath), "path": "/project"},
							})
						}
						updated = true
					}
				}
			}
		}
	}
	return updated, err
}

func CleanProjectPath(path string) (string, error) {
	var err error
	if !filepath.IsAbs(path) {
		return "", ErrNoRelativePathsAllowed
	}

	path, err = EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return path, nil
}
