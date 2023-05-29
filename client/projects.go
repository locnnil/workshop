package client

import (
	"fmt"
	"net/url"
)

type ProjectResponse struct {
	Id   string `json:"id"`
	Path string `json:"path"`
}

func (client *Client) ProjectId(path string) (string, error) {
	var projects []ProjectResponse
	query := url.Values{}
	query.Set("path", path)
	_, err := client.doSync("GET", "/v1/projects", query, nil, nil, &projects)
	if err != nil {
		return "", err
	}

	// if everything goes as planned, there will be an array with one element
	// describing the required project at the path
	if len(projects) == 1 {
		return projects[0].Id, nil
	}

	return "", fmt.Errorf("cannot get an unambigous project id for %q", path)
}
