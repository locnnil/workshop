package workshopbackend

import (
	"encoding/json"

	"github.com/canonical/workshop/internal/sdk"
)

func InstalledContent(lxdConfig map[string]string) (map[string]sdk.Setup, error) {
	content := make(map[string]sdk.Setup)
	if sdks, ok := lxdConfig["user.workshop.content"]; ok {
		err := json.Unmarshal([]byte(sdks), &content)
		if err != nil {
			return nil, err
		}
	}
	return content, nil
}
