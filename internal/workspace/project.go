package workspace

import (
	"encoding/hex"
	"errors"
	"math/rand"
	"path/filepath"
	"regexp"

	util "github.com/canonical/workspace/internal"
	srv "github.com/canonical/workspace/internal/server"
	"golang.org/x/exp/maps"

	"github.com/spf13/afero"
)

type Project struct {
	path      string
	projectId string

	fs     afero.Fs
	server srv.WorkspaceServer
}

const ProjectLock = ".workspace.lock"
const ProjectDevice = "workspace.project"

var validWorkspaceFilename = regexp.MustCompile(`^\.workspace\.(?P<name>[a-z_][a-z0-9_-]*)\.yaml$`)

var ErrNoRelativePathsAllowed = errors.New("relative paths are not allowed to refer to a project")
var ErrProjectDirectoryNotFound = errors.New("project directory does not exist")
var ErrProjectFileNotFound = errors.New(".lock file does not exist")

func LoadProjectFromInstances(server srv.WorkspaceServer, fs afero.Fs, workspaces []*srv.WorkspaceProps, path string) (*Project, error) {
	var project Project

	project.fs = fs
	project.server = server
	project.path = path

	if dirOk, err := afero.Exists(fs, path); err == nil {
		if !dirOk {
			/* Check the project bind-mount of the workspace
			to see whether the project was moved, renamed or deleted */
			for _, i := range workspaces {
				if i.State == util.Ready {
					done, err := server.Exec(i.Name, i.Devices[ProjectDevice]["source"], "root", []string{})
					if err != nil {
						continue
					}
					<-done
				}
			}
		} else {
			/* if .lock exists then just load normally and move on */
			if err = project.ReadProject(path); err == ErrProjectFileNotFound {
				/* .lock file does not exist but should. Attempt to recover */
				for _, i := range workspaces {
					if id, ok := i.Config["user.workspace.project-id"]; ok {
						project.projectId = id
						if err = project.SaveProject(); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}

	return &project, nil
}

func LoadProject(server srv.WorkspaceServer, fs afero.Fs, path string) (*Project, error) {
	var err error
	var project Project

	/* Make sure the path is canonicalised */
	if path, err = cleanPath(path); err != nil {
		return nil, err
	}

	if ok, err := afero.Exists(fs, path); err == nil {
		if !ok {
			return nil, ErrProjectDirectoryNotFound
		}
	} else {
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
	} else if errors.Is(err, ErrProjectFileNotFound) {
		if ok, err := project.tryToRecover(); err != nil {
			return nil, err
		} else if !ok && err == nil {
			/* There is no project at a given location, perhaps never was */
			return nil, ErrProjectFileNotFound
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
	if path, err = cleanPath(path); err != nil {
		return nil, err
	}

	if ok, err := afero.Exists(fs, path); err == nil {
		if !ok {
			return nil, ErrProjectDirectoryNotFound
		}
	} else {
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

func cleanPath(path string) (string, error) {
	var err error
	if !filepath.IsAbs(path) {
		return "", ErrNoRelativePathsAllowed
	}

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return path, nil
}

func (p *Project) validateProjectDirectory(instances map[string]*srv.WorkspaceProps) error {
	var err error
	for _, i := range instances {
		if i.Devices[ProjectDevice]["source"] != p.path {
			/* The directory was copied elsewhere, we need to generate a new project-id to let the old project-id persist */
			if ok, _ := afero.Exists(p.fs, LockPath(i.Devices[ProjectDevice]["source"])); ok {
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
func (p *Project) tryToRecover() (bool, error) {
	instances, err := p.server.GetWorkspacesByDevices(func(devices map[string]map[string]string) bool {
		if mount, ok := devices["workspace.project"]; ok {
			if mount["source"] == p.path {
				return true
			}
		}
		return false
	})

	if err != nil {
		return false, err
	}

	if len(instances) > 0 {
		/* Found a previous .lock file, restore it */
		for _, i := range instances {
			if id, ok := i.Config["user.workspace.project-id"]; ok {
				p.projectId = id
				p.SaveProject()
				return true, nil
			}
		}
	}
	return false, nil
}

func LockPath(path string) string {
	return filepath.Join(path, ProjectLock)
}

func (w *Project) ProjectId() string {
	return w.projectId
}

func (w *Project) ProjectDirectory() string {
	return w.path
}

func (w *Project) UpdateProjectDirectory(path string) error {
	w.path = path
	return w.SaveProject()
}

func (w *Project) ReadProject(path string) error {
	var err error
	var buf []byte
	if buf, err = afero.ReadFile(w.fs, filepath.Join(path, ProjectLock)); err == nil {
		w.projectId = string(buf)
	} else if errors.Is(err, afero.ErrFileNotFound) {
		return ErrProjectFileNotFound
	}
	return err
}

func (w *Project) SaveProject() error {
	return afero.WriteFile(w.fs, filepath.Join(w.path, ProjectLock), []byte(w.projectId), 0644)
}

func (w *Project) EnumWorkspaces() ([]*srv.WorkspaceProps, error) {
	/* (1) Find all the project's workspace files */
	workspaces, err := w.enumWorkspaceFiles()
	if err != nil {
		return nil, err
	}

	/* (2) List all the project's instances */
	instances, err := w.enumWorkspaceInstances()
	if err != nil {
		return nil, err
	}

	/* (3) Merge both lists from (1) and (2) to build a list of workspaces with their states */
	result := make(map[string]*srv.WorkspaceProps, len(workspaces)+len(instances))
	for i, val := range workspaces {
		if inst, ok := instances[i]; !ok {
			/* We only have a file no instance */
			val.State = util.Inactive
		} else {
			/* Both a file and instance exists */
			val.State = inst.State
			delete(instances, i)
		}
		result[i] = val
	}

	/* Now, instances contains only orphaned workspaces, i.e. no file */
	for i, val := range instances {
		val.State = util.Error
		result[i] = val
	}

	return maps.Values(result), nil
}

func (w *Project) enumWorkspaceFiles() (map[string]*srv.WorkspaceProps, error) {
	files, err := afero.ReadDir(w.fs, w.path)
	if err != nil {
		return nil, err
	}

	var workspaces = make(map[string]*srv.WorkspaceProps, len(files))

	for _, info := range files {
		if info.IsDir() {
			continue
		}

		/* The first element in names will contain the workspace name if matched */
		if names := validWorkspaceFilename.FindStringSubmatch(info.Name()); names != nil {
			workspaces[names[1]] = &srv.WorkspaceProps{Name: names[1], State: util.Inactive}
		}
	}
	return workspaces, nil
}

func (w *Project) enumWorkspaceInstances() (map[string]*srv.WorkspaceProps, error) {
	instances, err := w.server.GetWorkspacesByConfig(srv.NewWorkspaceConfigFilter("user.workspace.project-id", w.ProjectId()))
	if err != nil {
		return instances, err
	}
	return instances, nil
}

func EnumAllWorkspaces(server srv.WorkspaceServer, fs afero.Fs) (map[*Project][]*srv.WorkspaceProps, error) {
	all, err := server.GetWorkspacesByConfig(srv.EveryWorkspace())
	if err != nil {
		return nil, err
	}

	/* Get a project path for every project */
	var projects = make(map[string][]*srv.WorkspaceProps, len(all))
	for _, i := range all {
		projectPath := i.Devices[ProjectDevice]["source"]
		projects[projectPath] = append(projects[projectPath], i)
	}

	/* Get a list of Project objects with workspaces */
	var fullList = make(map[*Project][]*srv.WorkspaceProps, len(projects))
	for path, instances := range projects {
		if project, err := LoadProjectFromInstances(server, fs, instances, path); err == nil {
			fullList[project] = instances
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
