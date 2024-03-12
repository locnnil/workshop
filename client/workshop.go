package client

import (
	"bytes"
	"encoding/json"
	"time"
)

type HealthCheck struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
	Code      string    `json:"code,omitempty"`
}

type Mount struct {
	Plug   PlugRef `json:"plug"`
	Source string  `json:"source"`
	Target string  `json:"target"`
}

type Sdk struct {
	Name        string       `json:"name"`
	Channel     string       `json:"channel"`
	Revision    string       `json:"revision"`
	InstallTime time.Time    `json:"install-time"`
	Health      *HealthCheck `json:"health-check,omitempty"`
	Mounts      []*Mount     `json:"mounts,omitempty"`
}

type Workshop struct {
	ProjectId string   `json:"project-id"`
	Name      string   `json:"name"`
	Base      string   `json:"base"`
	Status    string   `json:"status"`
	Content   []*Sdk   `json:"content,omitempty"`
	Notes     []string `json:"notes,omitempty"`
}

type ListOptions struct {
	ProjectId string
}

type PlugRef struct {
	ProjectId string `json:"project-id"`
	Workshop  string `json:"workshop"`
	Sdk       string `json:"sdk"`
	Name      string `json:"plug"`
}

type Remount struct {
	Action string   `json:"action"`
	Plug   *PlugRef `json:"plug"`
	Source string   `json:"source"`
}

func (client *Client) ListWorkshops(opts *ListOptions) ([]*Workshop, error) {
	var workshops []*Workshop
	_, err := client.doSync("GET", "/v1/projects/"+opts.ProjectId+"/workshops", nil, nil, nil, &workshops)
	if err != nil {
		return nil, err
	}
	return workshops, nil
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
		Action: "remount",
		Plug:   plug,
		Source: source,
	}
	if err := json.NewEncoder(&body).Encode(remoutReq); err != nil {
		return "", err
	}

	return client.doAsync("POST", "/v1/projects/"+plug.ProjectId+"/workshops/"+plug.Workshop+"/mounts", nil, nil, &body)
}
