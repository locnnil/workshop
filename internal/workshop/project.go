package workshop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
)

var (
	ErrProjectNotFound        = errors.New("project not found")
	ErrNotAProject            = errors.New("not a project (no workshop files found)")
	ErrNoRelativePathsAllowed = errors.New("absolute project path must be used")
)

const (
	ProjectLock = ".workshop.lock"
	// path used in workshop to mount the project directory
	WorkshopProjectPath = "/project"
)

func LockPath(path string) string {
	return filepath.Join(path, ProjectLock)
}

type Project struct {
	Path      string `json:"path"`
	ProjectId string `json:"id"`
}

func (p *Project) Exists() bool {
	exists, dir, _ := osutil.ExistsIsDir(p.Path)
	return exists && dir
}

func (w *Project) Workshop(workshop string) (*WorkshopFile, error) {
	return readWorkshop(filepath.Join(w.Path, fmt.Sprintf(".workshop.%s.yaml", workshop)))
}

func (w *Project) ReadWorkshops() ([]*WorkshopFile, error) {
	files, err := os.ReadDir(w.Path)
	if err != nil {
		return nil, err
	}

	var workshops = make([]*WorkshopFile, 0, len(files))

	for _, info := range files {
		if info.IsDir() || !info.Type().IsRegular() {
			continue
		}

		// The first element in names will contain the workshop name if matched
		if names := validWorkshopFilename.FindStringSubmatch(info.Name()); names != nil {
			f, err := readWorkshop(filepath.Join(w.Path, info.Name()))
			if err != nil {
				logger.Noticef("Cannot parse %s: %v", info.Name(), err)
				continue
			}
			workshops = append(workshops, f)
		}
	}
	return workshops, nil
}

func (w *Project) updateProjectLock() error {
	lock, err := osutil.NewFileLockWithMode(LockPath(w.Path), 0644)
	if err != nil {
		return err
	}
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Close()

	_, err = lock.File().Write([]byte(w.ProjectId))
	if err != nil {
		return err
	}

	return lock.File().Sync()
}

func (w *Project) createProjectLock() error {
	lock, err := osutil.NewFileLockWithMode(LockPath(w.Path), 0644)
	if err != nil {
		return err
	}
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Close()

	id, err := io.ReadAll(lock.File())
	if err != nil {
		return err
	}

	if len(id) > 0 {
		return fmt.Errorf("project already exists")
	}

	_, err = lock.File().Write([]byte(w.ProjectId))
	if err != nil {
		return err
	}

	return lock.File().Sync()
}

func readProjects(jsonData []byte) ([]*Project, error) {
	var projects = make([]*Project, 0)
	if len(jsonData) == 0 {
		return projects, nil
	}
	if err := json.Unmarshal([]byte(jsonData), &projects); err != nil {
		return nil, fmt.Errorf("invalid projects record: %w", err)
	}
	return projects, nil
}

func saveProjects(projects []*Project) (string, error) {
	buf, err := json.Marshal(projects)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// Read a project id from projectDir (.workshop.lock)
func projectId(projectDir string) (string, error) {
	lock, err := osutil.OpenExistingLockForReading(LockPath(projectDir))
	if err != nil {
		return "", err
	}

	if err := lock.ReadLock(); err != nil {
		return "", err
	}

	defer lock.Close()
	buf, err := io.ReadAll(lock.File())
	if err != nil {
		return "", err
	}
	return string(buf), nil
}
