package workspacebackend

import (
	"errors"
	"os"

	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"
)

type WorkspaceFs interface {
	afero.Fs
	Symlink(old, new string, force bool) error
	Close()
}

func NewWorkspaceFs(c *sftp.Client) WorkspaceFs {
	var fs InstanceFs
	fs.client = c
	fs.Fs = sftpfs.New(fs.client)
	return &fs
}

type InstanceFs struct {
	afero.Fs
	client *sftp.Client
}

func (w *InstanceFs) Symlink(source, target string, force bool) error {
	if force {
		err := w.Remove(target)
		if errors.Is(err, afero.ErrFileNotFound) {
			return w.client.Symlink(source, target)
		} else {
			return err
		}
	}
	return w.client.Symlink(source, target)
}

func (w *InstanceFs) Close() {
	w.client.Close()
}

/* Fake wokrspace fs implementation for tests */

type FakeInstanceFs struct {
	afero.Fs
}

func NewFakeWorkspaceFs() WorkspaceFs {
	var fs FakeInstanceFs
	fs.Fs = afero.NewMemMapFs()
	return &fs
}

func (w *FakeInstanceFs) Symlink(source, target string, force bool) error {
	if force {
		err := w.Remove(target)
		if errors.Is(err, afero.ErrFileNotFound) {
			return w.Fs.Mkdir(target, os.ModeSymlink)
		} else {
			return err
		}
	}
	return w.Fs.Mkdir(target, os.ModeSymlink)
}

func (w *FakeInstanceFs) Close() {
}
