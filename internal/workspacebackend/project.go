package workspacebackend

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

var (
	ErrProjectNotFound        = errors.New("project not found")
	ErrNotAProject            = errors.New("not a project (no workspace files found)")
	ErrNoRelativePathsAllowed = errors.New("absolute project path must be used")
)

const (
	ProjectLock       = ".workspace.lock"
	ProjectPathDevice = "workspace.project"
	ProjectIdConfig   = "user.workspace.project-id"
)

var validWorkspaceFilename = regexp.MustCompile(`^\.workspace\.(?P<name>[a-z_][a-z0-9_-]*)\.yaml$`)

type Project struct {
	Path      string `json:"path"`
	ProjectId string `json:"id"`
}

func LockPath(path string) string {
	return filepath.Join(path, ProjectLock)
}

func projectId(lockFile string) (string, error) {
	if buf, err := os.ReadFile(lockFile); err == nil {
		return string(buf), nil
	} else {
		return "", err
	}
}

func (w *Project) UpdateLockFile() error {
	return os.WriteFile(LockPath(w.Path), []byte(w.ProjectId), 0644)
}

func (w *Project) WorkspaceFile(workspace string) (*WorkspaceFile, error) {
	file, err := ReadWorkspace(filepath.Join(w.Path, WorkspaceFileName(workspace)))
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (w *Project) EnumWorkspaceFiles() ([]*WorkspaceFile, error) {
	files, err := os.ReadDir(w.Path)
	if err != nil {
		return nil, err
	}

	var workspaces = make([]*WorkspaceFile, 0, len(files))

	for _, info := range files {
		if info.IsDir() || !info.Type().IsRegular() {
			continue
		}

		/* The first element in names will contain the workspace name if matched */
		if names := validWorkspaceFilename.FindStringSubmatch(info.Name()); names != nil {
			file, err := ReadWorkspace(filepath.Join(w.Path, info.Name()))
			if err != nil {
				return nil, err
			}

			workspaces = append(workspaces, file)
		}
	}
	return workspaces, nil
}
