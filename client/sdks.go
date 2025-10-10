package client

import "time"

// SdkVolume represents SDK volume summary returned by the daemon.
type SdkVolume struct {
	Name      string     `json:"name"`
	Version   string     `json:"version,omitempty"`
	Revision  string     `json:"revision"`
	BuildTime *time.Time `json:"build-time,omitempty"`
	Size      uint64     `json:"size,omitempty"`
}

type SdkInstalled struct {
	ProjectPath string `json:"project-path"`
	Workshop    string `json:"workshop"`
	Channel     string `json:"channel,omitempty"`
	SdkVolume
}

type SdkFullInfo struct {
	Name        string         `json:"name"`
	Summary     string         `json:"summary,omitempty"`
	Description string         `json:"description,omitempty"`
	Installed   []SdkInstalled `json:"installed,omitempty"`
}

// Sdks lists the SDK volumes known to the daemon.
func (client *Client) Sdks() ([]SdkVolume, error) {
	var sdks []SdkVolume
	_, err := client.doSync("GET", "/v1/sdks", nil, nil, nil, &sdks)
	if err != nil {
		return nil, err
	}
	return sdks, nil
}

// SdkInfo retrieves installation details of the given SDK across workshops.
func (client *Client) SdkInfo(name string) (*SdkFullInfo, error) {
	var info SdkFullInfo
	_, err := client.doSync("GET", "/v1/sdks/"+name, nil, nil, nil, &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}
