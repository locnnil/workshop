package server

import (
	"net/http"

	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
)

const SDK_PROJECT_NAME = "canonical.workspace"

/* Initialise the SDK project namespace. */
func initProject(conn lxd.InstanceServer) error {
	if _, _, err := conn.GetProject(SDK_PROJECT_NAME); err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return conn.CreateProject(api.ProjectsPost{
				ProjectPut: api.ProjectPut{
					Config: map[string]string{
						"features.images":          "true",
						"features.profiles":        "true",
						"features.storage.volumes": "true",
					},
					Description: "SDK Project Namespace",
				},
				Name: SDK_PROJECT_NAME,
			})
		} else {
			return err
		}
	}

	return nil
}
