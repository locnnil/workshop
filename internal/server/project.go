package server

import (
	"net/http"

	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
)

const WORKSPACE_PROJECT_NAME = "canonical.workspace"

/* Initialise the SDK project namespace. */
func initProject(conn lxd.InstanceServer) error {
	if _, _, err := conn.GetProject(WORKSPACE_PROJECT_NAME); err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return conn.CreateProject(api.ProjectsPost{
				ProjectPut: api.ProjectPut{
					Config: map[string]string{
						"features.images":          "true",
						"features.profiles":        "true",
						"features.storage.volumes": "true",
					},
					Description: "Workspace Project Namespace",
				},
				Name: WORKSPACE_PROJECT_NAME,
			})
		} else {
			return err
		}
	}

	return nil
}
