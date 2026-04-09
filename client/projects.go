package client

import (
	"bytes"
	"encoding/json"
	"net/url"
)

type Project struct {
	Id   string `json:"id"`
	Path string `json:"path"`
}

type WorkshopActionOptions struct {
	Mode string `json:"mode,omitempty"`
	// Supported refresh options: "update", "restore".
	RefreshOption string `json:"refresh-option,omitempty"`
	Verbose       bool   `json:"verbose,omitzero"`
}

type WorkshopActionSetup struct {
	Action  string
	Names   []string
	Options *WorkshopActionOptions
}

func (client *Client) Projects() ([]Project, error) {
	var projects []Project
	_, err := client.doSync("GET", "/v1/projects", nil, nil, nil, &projects)
	if err != nil {
		return nil, err
	}

	return projects, nil
}

func (client *Client) Project(path string) (*Project, error) {
	var project Project
	query := url.Values{}

	var postData struct {
		Path string `json:"path"`
	}
	postData.Path = path

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(postData); err != nil {
		return nil, err
	}

	_, err := client.doSync("POST", "/v1/projects", query, nil, &body, &project)
	if err != nil {
		return nil, err
	}

	return &project, nil
}

func (client *Client) doWorkshopAction(projectId string, action *WorkshopActionSetup) (changeId string, err error) {
	var postData struct {
		Names   []string               `json:"names"`
		Action  string                 `json:"action"`
		Options *WorkshopActionOptions `json:"options,omitempty"`
	}
	postData.Names = action.Names
	postData.Action = action.Action
	postData.Options = action.Options
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(postData); err != nil {
		return "", err
	}

	return client.doAsync("POST", "/v1/projects/"+projectId+"/workshops", nil, nil, &body)
}

func (client *Client) Launch(projectId string, names []string, mode string, verbose bool) (changeId string, err error) {
	return client.doWorkshopAction(projectId, &WorkshopActionSetup{
		Action: "launch",
		Names:  names,
		Options: &WorkshopActionOptions{
			Mode:    mode,
			Verbose: verbose,
		},
	})
}

func (client *Client) Refresh(projectId string, names []string, mode string, option string, verbose bool) (changeId string, err error) {
	return client.doWorkshopAction(projectId, &WorkshopActionSetup{
		Action: "refresh",
		Names:  names,
		Options: &WorkshopActionOptions{
			Mode:          mode,
			RefreshOption: option,
			Verbose:       verbose,
		},
	})
}

func (client *Client) Restore(projectId string, names []string, verbose bool) (changeId string, err error) {
	return client.doWorkshopAction(projectId, &WorkshopActionSetup{
		Action: "refresh",
		Names:  names,
		Options: &WorkshopActionOptions{
			Mode:          "transactional",
			RefreshOption: "restore",
			Verbose:       verbose,
		},
	})
}

func (client *Client) Start(projectId string, names []string) (changeId string, err error) {
	return client.doWorkshopAction(projectId, &WorkshopActionSetup{
		Action: "start",
		Names:  names,
	})
}

func (client *Client) Stop(projectId string, names []string) (changeId string, err error) {
	return client.doWorkshopAction(projectId, &WorkshopActionSetup{
		Action: "stop",
		Names:  names,
	})
}

func (client *Client) Remove(projectId string, names []string) (changeId string, err error) {
	return client.doWorkshopAction(projectId, &WorkshopActionSetup{
		Action: "remove",
		Names:  names,
	})
}
