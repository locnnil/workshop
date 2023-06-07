package client

import (
	"bytes"
	"encoding/json"
	"net/url"
)

type ProjectResponse struct {
	Id   string `json:"id"`
	Path string `json:"path"`
}

func (client *Client) Project(path string) (*ProjectResponse, error) {
	var project ProjectResponse
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
