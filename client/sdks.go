package client

// SdkVolume describes an SDK storage volume available on the system.
type SdkVolume struct {
	Name     string `json:"name"`
	Revision string `json:"revision"`
	Version  string `json:"version,omitempty"`
	Summary  string `json:"summary,omitempty"`
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
