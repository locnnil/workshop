package workspace

import (
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"regexp"

	util "github.com/canonical/workspace/internal"
	srv "github.com/canonical/workspace/internal/server"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
)

type Project struct {
	Path      string `yaml:"path"`
	ProjectId string `yaml:"project-id"`

	fs     afero.Fs
	server srv.WorkspaceServer
}

const PROJECT_FILE_NAME = ".workspace.lock"

var validWorkspaceFilename = regexp.MustCompile(`^\.workspace\.(?P<name>[a-z_][a-z0-9_-]*)\.yaml$`)

var ErrNoRelativePathsAllowed = errors.New("relative paths are not allowed to refer to a project")

func NewProject(server srv.WorkspaceServer, fs afero.Fs, path string) (*Project, error) {
	var err error
	var buf []byte
	var project Project
	project.fs = fs
	project.server = server

	/* Make sure the path is canonicalised */
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}

	if !filepath.IsAbs(path) {
		return nil, ErrNoRelativePathsAllowed
	}

	/* Is there an existing project file? */
	if buf, err = afero.ReadFile(fs, filepath.Join(path, PROJECT_FILE_NAME)); err == nil {
		if err = yaml.Unmarshal(buf, &project); err != nil {
			return nil, err
		}
	} else if errors.Is(err, afero.ErrFileNotFound) {
		/* The project did not exists before, initialise, but do not create a project file yet */
		project.Path = path
		project.ProjectId = fmt.Sprintf("%d", rand.Int63())
	} else {
		return nil, err
	}

	/* Project's directory was changed. We need to update its file and the instances' configuration
	to maintain integrity */
	/* TODO: make sure we compare apples to apples in terms of paths here */
	if project.GetProjectDirectory() != path {
		project.UpdateProjectDirectory(path)

		var prjMount = srv.WorkspaceDevice{
			Name:       srv.PROJECT_DEVICE_NAME,
			Properties: map[string]string{"type": "disk", "source": path, "path": "/project"},
		}
		server.AddWorkspacesDevice(srv.NewWorkspaceFilter("user.workspace.project-id", project.GetProjectId()), prjMount)
	}

	return &project, nil
}

func (w *Project) GetProjectId() string {
	return w.ProjectId
}

func (w *Project) GetProjectDirectory() string {
	return w.Path
}

func (w *Project) Exists() bool {
	ok, _ := afero.Exists(w.fs, filepath.Join(w.Path, PROJECT_FILE_NAME))
	return ok
}

func (w *Project) UpdateProjectDirectory(path string) error {
	w.Path = path
	return w.SaveProject()
}

func (w *Project) SaveProject() error {
	var buf []byte
	var err error
	if buf, err = yaml.Marshal(w); err != nil {
		return err
	}

	return afero.WriteFile(w.fs, filepath.Join(w.Path, PROJECT_FILE_NAME), buf, 0644)
}

func (w *Project) EnumWorkspaces() (map[string]srv.WorkspaceProps, error) {
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
	result := make(map[string]srv.WorkspaceProps, len(workspaces)+len(instances))
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
		val.State = util.Orphaned
		result[i] = val
	}

	return result, nil
}

func (w *Project) enumWorkspaceFiles() (map[string]srv.WorkspaceProps, error) {
	files, err := afero.ReadDir(w.fs, w.Path)
	if err != nil {
		return nil, err
	}

	var workspaces = make(map[string]srv.WorkspaceProps, len(files))

	for _, info := range files {
		if info.IsDir() {
			continue
		}

		/* The first element in names will contain the workspace name if matched */
		if names := validWorkspaceFilename.FindStringSubmatch(info.Name()); names != nil {
			workspaces[names[1]] = srv.WorkspaceProps{Name: names[1]}
		}
	}
	return workspaces, nil
}

func (w *Project) enumWorkspaceInstances() (map[string]srv.WorkspaceProps, error) {
	instances, err := w.server.GetWorkspaces(srv.NewWorkspaceFilter("user.workspace.project-id", w.GetProjectId()))
	if err != nil {
		return instances, err
	}
	return instances, nil
}

func (w *Project) EnumAllWorkspaces() (map[string]srv.WorkspaceProps, error) {
	workspaces, err := w.server.GetWorkspaces(srv.NoWorkspaceFilter())
	if err != nil {
		return nil, err
	}
	return workspaces, err
}
