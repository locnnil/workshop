package sdk

import (
	"path/filepath"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/osutil"
)

// OpenLock creates and opens a lock file associated with a particular SDK file.
func OpenLock(sdkName string) (*osutil.FileLock, error) {
	flock, err := osutil.NewFileLock(filepath.Join(dirs.WorkshopdLocksDir, sdkName+".lock"))
	if err != nil {
		return nil, err
	}
	return flock, nil
}
