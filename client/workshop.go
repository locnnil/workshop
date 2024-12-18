package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

type HealthCheck struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
	Code      string    `json:"code,omitempty"`
}

type Mount struct {
	Plug           PlugRef `json:"plug"`
	HostSource     string  `json:"host-source,omitempty"`
	WorkshopSource string  `json:"workshop-source,omitempty"`
	WorkshopTarget string  `json:"workshop-target"`
}

type Sdk struct {
	Name        string       `json:"name"`
	Channel     string       `json:"channel"`
	Revision    string       `json:"revision"`
	InstallTime time.Time    `json:"install-time"`
	Health      *HealthCheck `json:"health-check,omitempty"`
	Mounts      []*Mount     `json:"mounts,omitempty"`
}

type Workshops struct {
	Workshops []*WorkshopInfo `json:"workshops"`
	Files     []*WorkshopFile `json:"files"`
}

type WorkshopInfo struct {
	ProjectId string   `json:"project-id"`
	Name      string   `json:"name"`
	Base      string   `json:"base"`
	Status    string   `json:"status"`
	Content   []*Sdk   `json:"content,omitempty"`
	Notes     []string `json:"notes,omitempty"`
}

type WorkshopFile struct {
	ProjectId string `json:"project-id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
}

type Workshop struct {
	WorkshopInfo
	Path string `json:"path"`
}

type ListOptions struct {
	ProjectId string
}

type Remount struct {
	Action     string   `json:"action"`
	Plug       *PlugRef `json:"plug"`
	HostSource string   `json:"host-source"`
}

func (client *Client) List(opts *ListOptions) ([]*WorkshopInfo, []*WorkshopFile, error) {
	query := url.Values{}
	query.Set("state", "available")
	var info Workshops
	_, err := client.doSync("GET", "/v1/projects/"+opts.ProjectId+"/workshops", query, nil, nil, &info)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot list workshops: %w", err)
	}
	return info.Workshops, info.Files, nil
}

func (client *Client) Workshop(projectId, name string) (*Workshop, error) {
	var workshop Workshop
	_, err := client.doSync("GET", "/v1/projects/"+projectId+"/workshops/"+name, nil, nil, nil, &workshop)
	if err != nil {
		return nil, err
	}
	return &workshop, nil
}

func (client *Client) Remount(plug *PlugRef, source string) (changeId string, err error) {
	var body bytes.Buffer
	var remoutReq = Remount{
		Action:     "remount",
		Plug:       plug,
		HostSource: source,
	}
	if err := json.NewEncoder(&body).Encode(remoutReq); err != nil {
		return "", err
	}

	return client.doAsync("POST", "/v1/projects/"+plug.ProjectId+"/workshops/"+plug.Workshop+"/mounts", nil, nil, &body)
}
