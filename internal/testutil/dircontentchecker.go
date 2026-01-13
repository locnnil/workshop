// Copyright (c) 2014-2020 Canonical Ltd
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

package testutil

import (
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"slices"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/osutil"
)

type dirContentChecker struct {
	*check.CheckerInfo
}

// DirEquals verifies that the given directory contains exactly the given contents.
// Contents should be a slice of strings of the form "drwxr-xr-x filename".
var DirEquals check.Checker = &dirContentChecker{
	CheckerInfo: &check.CheckerInfo{Name: "DirEquals", Params: []string{"directory", "contents"}},
}

func (c *dirContentChecker) Check(params []any, names []string) (bool, string) {
	var infos []os.FileInfo
	switch dir := params[0].(type) {
	case string:
		recs, err := os.ReadDir(dir)
		if err != nil {
			return false, fmt.Sprintf("Cannot read directory %q: %v", dir, err)
		}
		infos, err = osutil.DirInfos(recs)
		if err != nil {
			return false, fmt.Sprintf("Cannot read directory %q: %v", dir, err)
		}
	case fs.ReadDirFile:
		recs, err := dir.ReadDir(-1)
		if err != nil {
			info, err1 := dir.Stat()
			if err1 != nil {
				return false, fmt.Sprintf("Cannot read directory: %v", err)
			}
			return false, fmt.Sprintf("Cannot read directory %q: %v", info.Name(), err)
		}
		infos, err = osutil.DirInfos(recs)
		if err != nil {
			return false, fmt.Sprintf("Cannot read directory %q: %v", dir, err)
		}
		slices.SortFunc(infos, func(a, b os.FileInfo) int { return cmp.Compare(a.Name(), b.Name()) })
	default:
		return false, "Directory must be a string or file handle"
	}

	obtained := make([]string, 0, len(infos))
	for _, info := range infos {
		obtained = append(obtained, fmt.Sprintf("%s %s", info.Mode().String(), info.Name()))
	}

	return DeepUnsortedMatches.Check([]any{obtained, params[1]}, names)
}
