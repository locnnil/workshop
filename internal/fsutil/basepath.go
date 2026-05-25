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

package fsutil

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/afero"

	"github.com/canonical/workshop/internal/revert"
)

type BasePathFile struct {
	afero.BasePathFile
}

func (f *BasePathFile) Chmod(mode os.FileMode) error {
	return f.File.(*os.File).Chmod(mode)
}

func (f *BasePathFile) Chown(uid, gid int) error {
	return f.File.(*os.File).Chown(uid, gid)
}

type BasePathFs struct {
	afero.BasePathFs
}

var _ FsBackend = (*BasePathFs)(nil)

func NewBasePathFs(path string) Fs {
	basepathfs := afero.NewBasePathFs(afero.NewOsFs(), path).(*afero.BasePathFs)
	return Fs{&BasePathFs{*basepathfs}}
}

func (f *BasePathFs) Close() error {
	return nil
}

func (f *BasePathFs) MkdirChmodChown(path string, perm os.FileMode, uid, gid int) error {
	rev := revert.New()
	defer rev.Fail()

	if err := f.Mkdir(path, perm); err != nil {
		return err
	}
	rev.Add(func() {
		_ = f.Remove(path)
	})

	if err := f.Chmod(path, perm); err != nil {
		return err
	}

	if err := f.Chown(path, uid, gid); err != nil {
		return err
	}

	rev.Success()
	return nil

}

func (f *BasePathFs) Open(path string) (File, error) {
	file, err := f.BasePathFs.Open(path)
	if file == nil {
		return nil, err
	}
	return &BasePathFile{*file.(*afero.BasePathFile)}, err
}

func (f *BasePathFs) OpenFile(path string, flag int, perm os.FileMode) (File, error) {
	file, err := f.BasePathFs.OpenFile(path, flag, perm)
	if file == nil {
		return nil, err
	}
	return &BasePathFile{*file.(*afero.BasePathFile)}, err
}

func (f *BasePathFs) Symlink(source, target string) error {
	return f.SymlinkIfPossible(source, target)
}

func (f *BasePathFs) ReadDir(path string) ([]os.FileInfo, error) {
	file, err := f.BasePathFs.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return file.Readdir(-1)
}

func (f *BasePathFs) Lstat(path string) (os.FileInfo, error) {
	info, possible, err := f.LstatIfPossible(path)
	if possible {
		return info, err
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Op == "lstat" {
		return info, err
	}
	return nil, fmt.Errorf("lstat not supported for path %q", path)
}

func (f *BasePathFs) Readlink(p string) (string, error) {
	return f.ReadlinkIfPossible(p)
}
