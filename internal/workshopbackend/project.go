package workshopbackend

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/canonical/workshop/internal/osutil"
)

var (
	ErrProjectNotFound        = errors.New("project not found")
	ErrNotAProject            = errors.New("not a project (no workshop files found)")
	ErrNoRelativePathsAllowed = errors.New("absolute project path must be used")
)

const (
	ProjectLock       = ".workshop.lock"
	ProjectPathDevice = "workshop.project"
	// path used in workshop to mount the project directory
	WorkshopProjectPath = "/project"
	ProjectIdConfig     = "user.workshop.project-id"
)

var validWorkshopFilename = regexp.MustCompile(`^\.workshop\.(?P<name>[a-z_][a-z0-9_-]*)\.yaml$`)

type Project struct {
	Path      string `json:"path"`
	ProjectId string `json:"id"`
}

func (p *Project) Exists() bool {
	exists, dir, _ := osutil.ExistsIsDir(p.Path)
	return exists && dir
}

func ReadProjects(jsonData []byte) ([]*Project, error) {
	var projects = make([]*Project, 0)
	if len(jsonData) == 0 {
		return projects, nil
	}
	if err := json.Unmarshal([]byte(jsonData), &projects); err != nil {
		return nil, fmt.Errorf("invalid projects record: %w", err)
	}
	return projects, nil
}

func SaveProjects(projects []*Project) (string, error) {
	buf, err := json.Marshal(projects)
	if err != nil {
		return "", err
	}
	return string(buf), nil
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

func (w *Project) WorkshopFile(workshop string) (*WorkshopFile, error) {
	file, err := ReadWorkshop(filepath.Join(w.Path, WorkshopFileName(workshop)))
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (w *Project) EnumWorkshopFiles() ([]*WorkshopFile, error) {
	files, err := os.ReadDir(w.Path)
	if err != nil {
		return nil, err
	}

	var workshops = make([]*WorkshopFile, 0, len(files))

	for _, info := range files {
		if info.IsDir() || !info.Type().IsRegular() {
			continue
		}

		/* The first element in names will contain the workshop name if matched */
		if names := validWorkshopFilename.FindStringSubmatch(info.Name()); names != nil {
			file, err := ReadWorkshop(filepath.Join(w.Path, info.Name()))
			if err != nil {
				return nil, err
			}

			workshops = append(workshops, file)
		}
	}
	return workshops, nil
}
