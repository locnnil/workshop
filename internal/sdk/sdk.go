package sdk

import (
	"fmt"
	"path/filepath"

	"github.com/canonical/workspace/internal/dirs"
)

type SdkInfo struct {
	Name     string `json:"name"`
	Channel  string `json:"channel"`
	Revision int64  `json:"revision"`
}

func (s *SdkInfo) Filename() string {
	return filepath.Join(dirs.SdkDir, fmt.Sprintf("%s_%d.sdk", s.Name, s.Revision))
}
