package workspacebackend

import (
	"net/http"

	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
)

/* Initialise the SDK project namespace. */
func InitProject(conn lxd.InstanceServer, projectName string) error {
	if _, _, err := conn.GetProject(projectName); err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return conn.CreateProject(api.ProjectsPost{
				ProjectPut: api.ProjectPut{
					Config: map[string]string{
						"features.images":          "true",
						"features.profiles":        "true",
						"features.storage.volumes": "true",
					},
				},
				Name: projectName,
			})
		} else {
			return err
		}
	}
	return nil
}
