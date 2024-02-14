package client

import "time"

type HealthCheck struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
	Code      string    `json:"code,omitempty"`
}

type Sdk struct {
	Name        string       `json:"name"`
	Channel     string       `json:"channel"`
	Revision    string       `json:"revision"`
	InstallTime time.Time    `json:"install-time"`
	Health      *HealthCheck `json:"health-check,omitempty"`
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
