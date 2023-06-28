package workspacebackend

import (
	"encoding/json"

	"github.com/canonical/workspace/internal/sdk"
)

func InstalledContent(lxdConfig map[string]string) (map[string]*sdk.SdkInfo, error) {
	content := make(map[string]*sdk.SdkInfo)
	if sdks, ok := lxdConfig["user.workspace.sdk"]; ok {
		err := json.Unmarshal([]byte(sdks), &content)
		if err != nil {
			return nil, err
		}
	}
	return content, nil
}
