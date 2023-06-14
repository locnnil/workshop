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

func (client *Client) Projects() ([]*Project, error) {
	var projects []*Project
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

func (client *Client) doWorkspaceAction(projectId string, actionName string, names []string) (changeId string, err error) {
	var postData struct {
		Names  []string `json:"names"`
		Action string   `json:"action"`
	}
	postData.Names = names
	postData.Action = actionName
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(postData); err != nil {
		return "", err
	}

	return client.doAsync("POST", "/v1/projects/"+projectId+"/workspaces", nil, nil, &body)
}

func (client *Client) Launch(projectId string, names []string) (changeId string, err error) {
	return client.doWorkspaceAction(projectId, "launch", names)
}
