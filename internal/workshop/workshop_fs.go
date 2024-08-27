package workshop

import (
	"os"

	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"
)

type WorkshopFs interface {
	afero.Fs
	Symlink(old, new string) error
	Close()
}

func NewWorkshopFs(c *sftp.Client) WorkshopFs {
	var fs InstanceFs
	fs.client = c
	fs.Fs = sftpfs.New(fs.client)
	return &fs
}

type InstanceFs struct {
	afero.Fs
	client *sftp.Client
}

func (w *InstanceFs) Symlink(source, target string) error {
	if _, err := w.client.Stat(target); err == nil {
		return os.ErrExist
	}
	return w.client.Symlink(source, target)
}

func (w *InstanceFs) Close() {
	w.client.Close()
}
