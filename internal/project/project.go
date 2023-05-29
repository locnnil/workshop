package project

import (
	"context"
	"encoding/hex"
	"errors"
	"math/rand"
	"path/filepath"
	"regexp"

	"golang.org/x/exp/slices"

	util "github.com/canonical/workspace/internal"
	backend "github.com/canonical/workspace/internal/workspacebackend"

	"github.com/spf13/afero"
)

type Project struct {
	Path      string `json:"path"`
	ProjectId string `json:"project-id"`

	fs      afero.Fs
	backend backend.WorkspaceBackend
}

const (
	ProjectLock        = ".workspace.lock"
	ProjectDeviceField = "workspace.project"
	ProjectIdField     = "user.workspace.project-id"
)

// for testing purposes
var (
	LoadProject = loadProject
)

var validWorkspaceFilename = regexp.MustCompile(`^\.workspace\.(?P<name>[a-z_][a-z0-9_-]*)\.yaml$`)

func loadProject(backend backend.WorkspaceBackend, fs afero.Fs, path string) (*Project, error) {
	var err error
	var project Project

	/* Make sure the path is canonicalised */
	if path, err = util.CleanProjectPath(path); err != nil {
		return nil, err
	}

	project.fs = fs
	project.backend = backend
	project.Path = path

	/* Is there an existing project file? */
	if err = project.ReadProject(); err == nil {
		/* See if the project's directory was changed. We need to update its file and the instances' configuration
		to maintain integrity */
		if err = project.validateProjectDirectory(); err != nil {
			return nil, err
		}
	} else if errors.Is(err, afero.ErrFileNotFound) {
		updateConfigFromBindMounts(backend, fs)
		if ok := project.recorverProjectId(); ok {
			/* recovered project-id successfully, recreate .lock */
			return &project, project.SaveProject()
		} else {
			/* There is no project at a given location, perhaps never was */
			return nil, err
		}
	} else {
		return nil, err
	}

	return &project, nil
}

func NewProject(backend backend.WorkspaceBackend, fs afero.Fs, path string) (*Project, error) {
	var err error
	var project Project

	/* Make sure the path is canonicalised */
	if path, err = util.CleanProjectPath(path); err != nil {
		return nil, err
	}

	project.fs = fs
	project.backend = backend
	project.Path = path

	if project.ProjectId, err = newProjectId(); err != nil {
		return nil, err
	}

	if err = project.SaveProject(); err != nil {
		return nil, err
	}

	return &project, nil
}

func (p *Project) validateProjectDirectory() error {
	var err error
	var updated bool

	instances, err := p.retrieveWorkspaceInstances()
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
	if updated, err = updateConfigFromBindMounts(p.backend, p.fs); err != nil {
		return err
	}

	if updated {
		/* the workspaces' configuration was updated, so re-fetch the instances of the project */
		instances, err = p.retrieveWorkspaceInstances()
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
				p.ProjectId, err = newProjectId()
				if err != nil {
					return err
				}
				return p.SaveProject()
			}

			/* The directory was moved, update the project mount */
			var mount = backend.WorkspaceDevice{
				Name:       ProjectDeviceField,
				Properties: map[string]string{"type": "disk", "source": p.Path, "path": "/project"},
			}
			p.backend.AddWorkspaceDevice(i.Name, p.ProjectId, mount)
		}

	}
	return nil
}

/*
Check if the project DID exist at this directory path and if so, try to recover it.

	We generate .lock file in the projects directory which could be easily deleted by a user
	after running smth like make clean given .lock is supposed to not be tracked by VCS. If such
	an accident happens the integrity of the previously created workspaces must not suffer. Here
	we attempt to recover a previously created .lock
*/
func (p *Project) recorverProjectId() bool {
	instances, _ := p.backend.GetWorkspacesByDevices(func(devices map[string]map[string]string) bool {
		if mount, ok := devices[ProjectDeviceField]; ok {
			if mount["source"] == p.Path {
				return true
			}
		}
		return false
	})

	if len(instances) > 0 {
		/* Found a previous .lock file, restore it */
		for _, i := range instances {
			if id, ok := i.Config[ProjectIdField]; ok {
				p.ProjectId = id
				return true
			}
		}
	}
	return false
}

func (w *Project) Exists() bool {
	if ok, err := afero.Exists(w.fs, w.Path); err == nil {
		return ok
	}
	return false
}

func (w *Project) ReadProject() error {
	var err error
	var buf []byte
	if buf, err = afero.ReadFile(w.fs, filepath.Join(w.Path, ProjectLock)); err == nil {
		w.ProjectId = string(buf)
		return nil
	}
	return err
}

func (w *Project) SaveProject() error {
	return afero.WriteFile(w.fs, filepath.Join(w.Path, ProjectLock), []byte(w.ProjectId), 0644)
}

func (w *Project) RetrieveWorkspaces() ([]*backend.WorkspaceProps, error) {
	/* (1) Find all the project's workspace files */
	files, err := w.EnumWorkspaceFiles()
	if err != nil {
		return nil, err
	}

	/* (2) List all the project's instances */
	instances, err := w.retrieveWorkspaceInstances()
	if err != nil {
		return nil, err
	}

	result := mergeInstancesAndFiles(files, instances)

	return result, nil
}

func mergeInstancesAndFiles(files []*backend.WorkspaceProps, instances []*backend.WorkspaceProps) []*backend.WorkspaceProps {
	/* Merge both lists from to build a list of workspaces with their states */
	result := make([]*backend.WorkspaceProps, 0, len(files)+len(instances))
	for _, ws := range instances {
		finder := func(p *backend.WorkspaceProps) bool { return p.Name == ws.Name }
		idx := slices.IndexFunc(files, finder)
		if idx == -1 {
			/* We only have an instance, no file
			 */
			ws.SetState(util.Error, util.MissingFile)
		} else {
			/* Both a file and instance exist */
			files = slices.Delete(files, idx, idx+1)
		}
		result = append(result, ws)
	}

	/* Now, files contains only inactive workspaces */
	for _, ws := range files {
		ws.SetState(util.Off, util.None)
		result = append(result, ws)
	}
	return result
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

func (w *Project) retrieveWorkspaceInstances() ([]*backend.WorkspaceProps, error) {
	instances, err := w.backend.GetWorkspacesByConfig(backend.NewWorkspaceConfigFilter(ProjectIdField, w.ProjectId))
	if err != nil {
		return instances, err
	}
	return instances, nil
}

func RetrieveWorkspacesGlobal(be backend.WorkspaceBackend, fs afero.Fs) (map[*Project][]*backend.WorkspaceProps, error) {
	updateConfigFromBindMounts(be, fs)

	all, err := be.GetWorkspacesByConfig(backend.EveryWorkspace())
	if err != nil {
		return nil, err
	}

	/* Group by instances by project-id and project directory  */
	type projectKey struct {
		path, id string
	}
	var projects = make(map[projectKey]bool, len(all))
	for _, i := range all {
		projectPath := i.Devices[ProjectDeviceField]["source"]
		projectId := i.Config[ProjectIdField]
		projects[projectKey{path: projectPath, id: projectId}] = true
	}

	/* Get a list of Project objects with workspaces */
	var fullList = make(map[*Project][]*backend.WorkspaceProps, len(projects))
	for props := range projects {
		// in this case a .lock file is in the project directory and the project can be
		// loaded, which means, there are workspaces for this project and
		// the project directory exists
		if project, err := LoadProject(be, fs, props.path); err == nil {
			workspaces, err := project.RetrieveWorkspaces()
			if err == nil {
				fullList[project] = workspaces
			}
		} else if errors.Is(err, afero.ErrFileNotFound) {
			// all the workspaces of this project are unreachable and the directory
			// does not exist anymore. However, there could be stopped instances that are orphaned
			// we make sure these are not skipped in the output
			project = &Project{Path: props.path, ProjectId: props.id, backend: be, fs: fs}
			workspaces, err := project.retrieveWorkspaceInstances()
			if len(workspaces) > 0 && err == nil {
				for _, i := range workspaces {
					i.SetState(util.Error, util.MissingProject)
				}
				fullList[project] = workspaces
			}
		}
	}
	return fullList, nil
}

func newProjectId() (string, error) {
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
func updateConfigFromBindMounts(be backend.WorkspaceBackend, fs afero.Fs) (updated bool, err error) {
	workspaces, err := be.GetWorkspacesByConfig(backend.EveryWorkspace())
	if err != nil {
		return false, err
	}

	/* Group by instances by project-id and project directory  */
	type projectKey struct {
		path, id string
	}
	var grouped = make(map[projectKey][]*backend.WorkspaceProps, len(workspaces))
	for _, i := range workspaces {
		if i.State() == util.Ready {
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
		stdout, err := memFs.Create(util.ToInstanceName(instance.Name, key.id))
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

		ctx := context.WithValue(context.Background(), backend.ContextProjectId, key.id)
		done, err := be.Exec(ctx, instance.Name, &args)
		if err != nil {
			continue
		}
		<-done

		/* Process the findmnt results */
		if currentPath, err := afero.ReadFile(memFs, util.ToInstanceName(instance.Name, key.id)); err == nil {
			/* check if the path is not //deleted */
			if ok, _ := afero.Exists(fs, string(currentPath)); ok {
				if lxdPath, ok := instance.Devices[ProjectDeviceField]["source"]; ok {
					if lxdPath != string(currentPath) {
						/* now, update LXD configuration for all the group's instances */
						for _, inst := range i {
							be.AddWorkspaceDevice(inst.Name, key.id, backend.WorkspaceDevice{
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
