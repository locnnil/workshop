package sdk

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/canonical/workspace/internal/dirs"
)

const (
	WorkspaceSdksDir = "/var/lib/workspace/sdk"
)

type SdkInfo struct {
	Name        string    `json:"name"`
	Channel     string    `json:"channel"`
	Revision    int64     `json:"revision"`
	InstallTime time.Time `json:"install-time"`
}

func SdkCurrentPath(sdkName string) string {
	return filepath.Join(WorkspaceSdksDir, sdkName, "current")
}

func SdkHooksDir(sdkName string) string {
	return filepath.Join(SdkCurrentPath(sdkName), "hooks")
}

func SdkHookPath(sdkName, hookName string) string {
	return filepath.Join(SdkHooksDir(sdkName), hookName)
}

func (s *SdkInfo) Filename() string {
	return filepath.Join(dirs.SdkDir, fmt.Sprintf("%s_%d.sdk", s.Name, s.Revision))
}
