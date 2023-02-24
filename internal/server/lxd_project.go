package server

import (
	"fmt"
	"net/http"

	"os/user"

	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
)

const WORKSPACE_PROJECT_NAME_PREFIX string = "workspace."

func GetLXDProjectName() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}

	return WORKSPACE_PROJECT_NAME_PREFIX + user.Username, nil
}

/* Initialise the SDK project namespace. */
func InitProject(conn lxd.InstanceServer) error {
	project, err := GetLXDProjectName()
	if err != nil {
		return err
	}

	user, err := user.Current()
	if err != nil {
		return err
	}
	if _, _, err := conn.GetProject(project); err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return conn.CreateProject(api.ProjectsPost{
				ProjectPut: api.ProjectPut{
					Config: map[string]string{
						"features.images":          "true",
						"features.profiles":        "true",
						"features.storage.volumes": "true",
					},
					Description: fmt.Sprintf("%s's workspaces", user.Username),
				},
				Name: project,
			})
		} else {
			return err
		}
	}

	return nil
}
