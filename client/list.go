package client

type Sdk struct {
	Name     string `json:"name"`
	Channel  string `json:"channel"`
	Revision string `json:"revision"`
}

type Workspace struct {
	ProjectId string   `json:"project-id"`
	Name      string   `json:"name"`
	State     string   `json:"state"`
	Content   []*Sdk   `json:"content,omitempty"`
	Notes     []string `json:"notes,omitempty"`
}

type ListOptions struct {
	ProjectId string
}

func (client *Client) ListWorkspaces(opts *ListOptions) ([]*Workspace, error) {
	var workspaces []*Workspace
	_, err := client.doSync("GET", "/v1/projects/"+opts.ProjectId+"/workspaces", nil, nil, nil, &workspaces)
	if err != nil {
		return nil, err
	}
	return workspaces, nil
}
