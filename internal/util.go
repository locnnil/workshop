package util

import (
	"encoding/binary"
	"fmt"

	"os"
	"os/signal"
	"strings"

	"path/filepath"

	"github.com/adrg/xdg"
	lxd "github.com/lxc/lxd/client"

	"math/rand"

	crypto_rand "crypto/rand"
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
	Error
)

func (s WorkspaceState) String() string {
	return [...]string{"Inactive", "Ready", "Stopped", "Pending", "Error"}[s]
}

func ToFileName(name string) string {
	return fmt.Sprintf(".workspace.%s.yaml", name)
}

func ToInstanceName(name string, project_id string) string {
	return fmt.Sprintf("%s-%s", name, project_id)
}

func ToWorkspaceName(instance string) string {
	idx := strings.LastIndex(instance, "-")
	if idx == -1 {
		return ""
	}

	// drop the project id from the name
	return instance[:idx]
}

func init() {
	var b [8]byte
	_, err := crypto_rand.Read(b[:])
	if err != nil {
		panic("cannot seed math/rand package")
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))

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
