package workshop

import (
	"crypto/rand"
	"encoding/hex"
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

	NewProjectId = allocateProjectId
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

func (w *Project) Workshop(workshop string) (*File, error) {
	return readWorkshop(filepath.Join(w.Path, fmt.Sprintf(".workshop.%s.yaml", workshop)))
}

func (w *Project) ReadWorkshops() ([]*File, error) {
	files, err := os.ReadDir(w.Path)
	if err != nil {
		return nil, err
	}

	var workshops = make([]*File, 0, len(files))

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

func (w *Project) UpdateProjectLock() error {
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

func (w *Project) CreateProjectLock() error {
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

// Read a project id from projectDir (.workshop.lock)
func ProjectId(projectDir string) (string, error) {
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

func allocateProjectId() (string, error) {
	bytes := make([]byte, 4)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
