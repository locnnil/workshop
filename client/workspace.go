package client

import "time"

type Sdk struct {
	Name        string    `json:"name"`
	Channel     string    `json:"channel"`
	Revision    string    `json:"revision"`
	InstallTime time.Time `json:"install-time"`
}

type Workshop struct {
	ProjectId string   `json:"project-id"`
	Name      string   `json:"name"`
	Base      string   `json:"base"`
	State     string   `json:"state"`
	Content   []*Sdk   `json:"content,omitempty"`
	Notes     []string `json:"notes,omitempty"`
}

type ListOptions struct {
	ProjectId string
}

func (client *Client) ListWorkspaces(opts *ListOptions) ([]*Workshop, error) {
	var workspaces []*Workshop
	_, err := client.doSync("GET", "/v1/projects/"+opts.ProjectId+"/workspaces", nil, nil, nil, &workspaces)
	if err != nil {
		return nil, err
	}
	return workspaces, nil
}

func (client *Client) Workshop(projectId, name string) (*Workshop, error) {
	var workshop Workshop
	_, err := client.doSync("GET", "/v1/projects/"+projectId+"/workspaces/"+name, nil, nil, nil, &workshop)
	if err != nil {
		return nil, err
	}
	return &workshop, nil
}
