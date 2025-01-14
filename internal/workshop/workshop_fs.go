package workshop

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"

	"github.com/canonical/workshop/internal/revert"
)

type WorkshopFs interface {
	afero.Fs
	Symlink(old, new string) error
	ReadLink(p string) (string, error)
	Close()
}

func NewWorkshopFs(c *sftp.Client) *InstanceFs {
	var fs InstanceFs
	fs.client = c
	fs.Fs = sftpfs.New(fs.client)
	return &fs
}

type InstanceFs struct {
	afero.Fs
	client *sftp.Client
}

func (w *InstanceFs) RemoveAll(path string) error {
	return w.client.RemoveAll(path)
}

func (w *InstanceFs) Rename(oldname, newname string) error {
	return w.client.PosixRename(oldname, newname)
}

func (w *InstanceFs) Symlink(source, target string) error {
	if _, err := w.client.Stat(target); err == nil {
		return os.ErrExist
	}
	return w.client.Symlink(source, target)
}

func (w *InstanceFs) ReadLink(p string) (string, error) {
	return w.client.ReadLink(p)
}

func (w *InstanceFs) Close() {
	w.client.Close()
}

func AtomicWrite(fs afero.Fs, filename string, source io.WriterTo, perm os.FileMode) error {
	dir, name := filepath.Split(filename)
	// If dir is empty, TempFile uses os.TempDir() instead,
	// so rename might fail or be non-atomic.
	if dir == "" {
		return fmt.Errorf("parent directory not found for %q", filename)
	}

	file, err := afero.TempFile(fs, dir, name+".*~")
	if err != nil {
		return err
	}

	rev := revert.New()
	defer rev.Fail()

	temp := file.Name()
	rev.Add(func() { _ = fs.Remove(temp) })

	_, err = source.WriteTo(file)
	file.Close()
	if err != nil {
		return err
	}

	if err = fs.Chmod(temp, perm); err != nil {
		return err
	}

	if err = fs.Rename(temp, filename); err != nil {
		return err
	}

	rev.Success()
	return nil
}
