// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package client

import (
	"net/url"
	"time"
)

type StoreAccount struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display-name"`
	Validation  string `json:"validation,omitempty"`
}

type SdkRevision struct {
	Channel      string     `json:"channel"`
	Track        string     `json:"track"`
	Risk         string     `json:"risk"`
	Revision     string     `json:"revision"`
	BuiltAt      *time.Time `json:"built-at,omitempty"`
	UploadedAt   *time.Time `json:"uploaded-at,omitempty"`
	ReleasedAt   *time.Time `json:"released-at,omitempty"`
	Version      string     `json:"version,omitempty"`
	Base         string     `json:"base,omitempty"`
	Arch         string     `json:"arch,omitempty"`
	DownloadSize uint64     `json:"download-size,omitzero"`
}

// SdkVolume represents SDK volume summary returned by the daemon.
type SdkVolume struct {
	Name     string     `json:"name"`
	Version  string     `json:"version,omitempty"`
	Revision string     `json:"revision"`
	BuiltAt  *time.Time `json:"built-at,omitempty"`
	Size     uint64     `json:"size,omitempty"`
}

type SdkInstalled struct {
	ProjectPath string `json:"project-path"`
	Workshop    string `json:"workshop"`
	Channel     string `json:"channel,omitempty"`
	Base        string `json:"base,omitempty"`
	Arch        string `json:"architecture,omitempty"`
	SdkVolume
}

type SdkFullInfo struct {
	Name        string         `json:"name"`
	PackageID   string         `json:"package-id,omitempty"`
	Title       string         `json:"title,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	Description string         `json:"description,omitempty"`
	License     string         `json:"license,omitempty"`
	Publisher   *StoreAccount  `json:"publisher,omitempty"`
	Channels    []*SdkRevision `json:"channels,omitempty"`
	Installed   []SdkInstalled `json:"installed,omitempty"`
}

type SdkSummary struct {
	Name        string        `json:"name"`
	PackageID   string        `json:"package-id,omitempty"`
	Summary     string        `json:"summary,omitempty"`
	Description string        `json:"description,omitempty"`
	License     string        `json:"license,omitempty"`
	Publisher   *StoreAccount `json:"publisher,omitempty"`
	Channel     string        `json:"channel"`
	Track       string        `json:"track"`
	Risk        string        `json:"risk"`
	Revision    string        `json:"revision"`
	ReleasedAt  *time.Time    `json:"released-at,omitempty"`
	Version     string        `json:"version,omitempty"`
	Base        string        `json:"base,omitempty"`
	Arch        string        `json:"arch,omitempty"`
}

// FindSdks searches the SDK Store.
func (client *Client) FindSdks(q string) ([]SdkSummary, error) {
	var sdks []SdkSummary
	query := url.Values{}
	query.Set("q", q)

	_, err := client.doSync("GET", "/v1/find", query, nil, nil, &sdks)
	if err != nil {
		return nil, err
	}
	return sdks, nil
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
