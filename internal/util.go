package util

import (
	"fmt"
	"os"
	"os/signal"

	"path/filepath"

	"github.com/adrg/xdg"
	lxd "github.com/lxc/lxd/client"
)

var (
	ErrCancelled    = fmt.Errorf("LXD operation cancelled by user")
	ErrForcedCancel = fmt.Errorf("LXD operation forcefully cancelled by user")
)

var (
	DataDir, SdksDir, WorkspaceSdksDir string
)

type WorkspaceState int

const (
	Inactive WorkspaceState = iota
	Ready
	Stopped
	Pending
	Orphaned
)

func (s WorkspaceState) String() string {
	return [...]string{"inactive", "ready", "stopped", "pending", "orphaned"}[s]
}

func ToFileName(name string) string {
	return fmt.Sprintf(".workspace.%s.yaml", name)
}

func init() {
	xdg.Reload()
	DataDir = filepath.Join(xdg.DataHome, "workspace")
	SdksDir = filepath.Join(DataDir, "sdks")

	WorkspaceSdksDir = "/var/lib/workspace/sdks/"

	if err := os.MkdirAll(SdksDir, 0700); err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}
}

func CancellableWait(op lxd.RemoteOperation) error {
	sch := make(chan os.Signal, 1)
	och := make(chan error)

	signal.Notify(sch, os.Interrupt)

	go func() {
		och <- op.Wait()
		close(och)
	}()

	count := 0
	for {
		select {
		case err := <-och:
			return err
		case <-sch:
			if err := op.CancelTarget(); err == nil {
				return ErrCancelled
			}

			if count += 1; count >= 3 {
				return ErrForcedCancel
			}
		}
	}
}
