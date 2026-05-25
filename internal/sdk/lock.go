// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
