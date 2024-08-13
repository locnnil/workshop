package workshop

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
)

var (
	ErrProjectNotFound        = errors.New("project not found")
	ErrProjectLockNotFound    = errors.New("project lock file not found")
	ErrProjectAlreadyExists   = errors.New("project already exists")
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
		if names := filename.FindStringSubmatch(info.Name()); names != nil {
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
	return w.createLock()
}

func (w *Project) CreateProjectLock() error {
	if osutil.FileExists(LockPath(w.Path)) {
		return ErrProjectAlreadyExists
	}
	return w.createLock()
}

func (w *Project) createLock() error {
	lock, err := os.Create(LockPath(w.Path))
	if err != nil {
		return err
	}
	defer lock.Close()

	// get the desired ownership
	info, err := os.Stat(w.Path)
	if err != nil {
		return err
	}

	uid, gid := 0, 0
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uid = int(stat.Uid)
		gid = int(stat.Gid)
	}

	if err = os.Chown(LockPath(w.Path), uid, gid); err != nil {
		return err
	}

	_, err = lock.Write([]byte(w.ProjectId))
	if err != nil {
		return err
	}

	return nil
}

// Read a project id from projectDir (.workshop.lock)
func ProjectId(projectDir string) (string, error) {
	buf, err := os.ReadFile(LockPath(projectDir))
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
