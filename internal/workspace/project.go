package workspace

import (
	"encoding/hex"
	"errors"
	"math/rand"
	"path/filepath"
	"regexp"

	util "github.com/canonical/workspace/internal"
	srv "github.com/canonical/workspace/internal/server"
	"golang.org/x/exp/slices"

	"github.com/spf13/afero"
)

type Project struct {
	path      string
	projectId string

	fs     afero.Fs
	server srv.WorkspaceServer
}

const (
	ProjectLock   = ".workspace.lock"
	ProjectDevice = "workspace.project"
	ProjectId     = "user.workspace.project-id"
)

var validWorkspaceFilename = regexp.MustCompile(`^\.workspace\.(?P<name>[a-z_][a-z0-9_-]*)\.yaml$`)

func LoadProject(server srv.WorkspaceServer, fs afero.Fs, path string) (*Project, error) {
	var err error
	var project Project

	/* Make sure the path is canonicalised */
	if path, err = util.CleanProjectPath(path); err != nil {
		return nil, err
	}

	project.fs = fs
	project.server = server
	project.path = path

	/* Is there an existing project file? */
	if err = project.ReadProject(path); err == nil {
		/* See if the project's directory was changed. We need to update its file and the instances' configuration
		to maintain integrity */
		instances, err := project.enumWorkspaceInstances()
		if err != nil {
			return nil, err
		}
		if err = project.validateProjectDirectory(instances); err != nil {
			return nil, err
		}
	} else if errors.Is(err, afero.ErrFileNotFound) {
		UpdateConfigFromBindMounts(server, fs)
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

func NewProject(server srv.WorkspaceServer, fs afero.Fs, path string) (*Project, error) {
	var err error
	var project Project

	/* Make sure the path is canonicalised */
	if path, err = util.CleanProjectPath(path); err != nil {
		return nil, err
	}

	project.fs = fs
	project.server = server
	project.path = path

	if project.projectId, err = newProjectId(); err != nil {
		return nil, err
	}

	return &project, nil
}

func (p *Project) validateProjectDirectory(instances []*srv.WorkspaceProps) error {
	var err error
	for _, i := range instances {
		source := i.Devices[ProjectDevice]["source"]
		if source != p.path {
			/* The directory was copied elsewhere, we need to generate a new project-id to let the old project-id persist */
			if ok, _ := afero.Exists(p.fs, LockPath(source)); ok {
				p.projectId, err = newProjectId()
				if err != nil {
					return err
				}
				return p.SaveProject()
			}

			/* The directory was moved, update all instances project mounts */
			var mount = srv.WorkspaceDevice{
				Name:       ProjectDevice,
				Properties: map[string]string{"type": "disk", "source": p.path, "path": "/project"},
			}
			p.server.AddWorkspaceDevice(i.Name, p.projectId, mount)
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
	instances, _ := p.server.GetWorkspacesByDevices(func(devices map[string]map[string]string) bool {
		if mount, ok := devices[ProjectDevice]; ok {
			if mount["source"] == p.path {
				return true
			}
		}
		return false
	})

	if len(instances) > 0 {
		/* Found a previous .lock file, restore it */
		for _, i := range instances {
			if id, ok := i.Config[ProjectId]; ok {
				p.projectId = id
				return true
			}
		}
	}
	return false
}

func (w *Project) ProjectId() string {
	return w.projectId
}

func (w *Project) ProjectDirectory() string {
	return w.path
}

func (w *Project) Exists() bool {
	if ok, err := afero.Exists(w.fs, w.path); err == nil {
		return ok
	}
	return false
}

func (w *Project) ReadProject(path string) error {
	var err error
	var buf []byte
	if buf, err = afero.ReadFile(w.fs, filepath.Join(path, ProjectLock)); err == nil {
		w.projectId = string(buf)
		return nil
	}
	return err
}

func (w *Project) SaveProject() error {
	return afero.WriteFile(w.fs, filepath.Join(w.path, ProjectLock), []byte(w.projectId), 0644)
}

func (w *Project) EnumWorkspaces() ([]*srv.WorkspaceProps, error) {
	/* (1) Find all the project's workspace files */
	files, err := w.EnumWorkspaceFiles()
	if err != nil {
		return nil, err
	}

	/* (2) List all the project's instances */
	instances, err := w.enumWorkspaceInstances()
	if err != nil {
		return nil, err
	}

	result := mergeInstancesAndFiles(files, instances)

	return result, nil
}

func mergeInstancesAndFiles(files []*srv.WorkspaceProps, instances []*srv.WorkspaceProps) []*srv.WorkspaceProps {
	/* Merge both lists from to build a list of workspaces with their states */
	result := make([]*srv.WorkspaceProps, 0, len(files)+len(instances))
	for _, ws := range files {
		finder := func(p *srv.WorkspaceProps) bool { return p.Name == ws.Name }
		idx := slices.IndexFunc(instances, finder)
		if idx == -1 {
			/* We only have a file no instance which
			we won't include in the final output
			*/
			ws.State = util.Inactive
			continue
		} else {
			/* Both a file and instance exists */
			ws.State = instances[idx].State
			instances = slices.Delete(instances, idx, idx+1)
		}
		result = append(result, ws)
	}

	/* Now, instances contains only orphaned workspaces, i.e. no file */
	for _, ws := range instances {
		ws.State = util.Error
		result = append(result, ws)
	}
	return result
}

func (w *Project) EnumWorkspaceFiles() ([]*srv.WorkspaceProps, error) {
	files, err := afero.ReadDir(w.fs, w.path)
	if err != nil {
		return nil, err
	}

	var workspaces = make([]*srv.WorkspaceProps, 0, len(files))

	for _, info := range files {
		if info.IsDir() {
			continue
		}

		/* The first element in names will contain the workspace name if matched */
		if names := validWorkspaceFilename.FindStringSubmatch(info.Name()); names != nil {
			workspaces = append(workspaces, &srv.WorkspaceProps{Name: names[1], State: util.Inactive})
		}
	}
	return workspaces, nil
}

func (w *Project) enumWorkspaceInstances() ([]*srv.WorkspaceProps, error) {
	instances, err := w.server.GetWorkspacesByConfig(srv.NewWorkspaceConfigFilter(ProjectId, w.ProjectId()))
	if err != nil {
		return instances, err
	}
	return instances, nil
}

func EnumWorkspacesGlobal(server srv.WorkspaceServer, fs afero.Fs) (map[*Project][]*srv.WorkspaceProps, error) {
	UpdateConfigFromBindMounts(server, fs)

	all, err := server.GetWorkspacesByConfig(srv.EveryWorkspace())
	if err != nil {
		return nil, err
	}

	type ProjectProps struct {
		path      string
		projectId string
	}

	/* Get a project path for every project */
	var projects = make(map[ProjectProps]bool, len(all))
	for _, i := range all {
		projectPath := i.Devices[ProjectDevice]["source"]
		projects[ProjectProps{path: projectPath, projectId: i.Config[ProjectId]}] = true
	}

	/* Get a list of Project objects with workspaces */
	var fullList = make(map[*Project][]*srv.WorkspaceProps, len(projects))
	for props := range projects {
		if project, err := LoadProject(server, fs, props.path); err == nil {
			workspaces, err := project.EnumWorkspaces()
			if err == nil {
				fullList[project] = workspaces
			}
		} else if errors.Is(err, afero.ErrFileNotFound) {
			// all the workspaces of this project are unreachable and the directory
			// does not exist anymore. However, there could be stopped instances that are orphaned
			// we make sure these are not skipped in the output
			project = &Project{path: props.path, projectId: props.projectId, server: server, fs: fs}
			workspaces, err := project.enumWorkspaceInstances()
			if len(workspaces) > 0 && err == nil {
				for _, i := range workspaces {
					i.State = util.Error
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

func UpdateConfigFromBindMounts(server srv.WorkspaceServer, fs afero.Fs) error {
	workspaces, err := server.GetWorkspacesByConfig(srv.EveryWorkspace())
	if err != nil {
		return err
	}

	memFs := afero.NewMemMapFs()
	for _, i := range workspaces {
		if projectId, ok := i.Config[ProjectId]; ok && i.State == util.Ready {
			stdout, err := memFs.Create(util.ToInstanceName(i.Name, projectId))
			if err != nil {
				return err
			}

			args := srv.ExecArgs{User: "root", Command: []string{"bash", "-c",
				"findmnt --mountpoint /project -o source -n | awk -F\"[][]\" '{printf $2}'"},
				Stdin: nil, Stdout: stdout, Stderr: nil}
			done, err := server.Exec(i.Name, projectId, &args)
			if err != nil {
				continue
			}
			<-done

			if currentPath, err := afero.ReadFile(memFs, util.ToInstanceName(i.Name, projectId)); err == nil {
				if ok, _ := afero.Exists(fs, string(currentPath)); ok {
					if lxdPath, ok := i.Devices[ProjectDevice]["source"]; ok {
						if lxdPath != string(currentPath) {
							server.AddWorkspaceDevice(i.Name, projectId, srv.WorkspaceDevice{
								Name:       ProjectDevice,
								Properties: map[string]string{"type": "disk", "source": string(currentPath), "path": "/project"},
							})
						}
					}
				}
			}

		}
	}
	return nil
}
