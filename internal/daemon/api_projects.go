package daemon

import (
	"net/http"

	"github.com/canonical/workspace/internal/project"
	"github.com/spf13/afero"
)

func v1Projects(c *Command, r *http.Request, _ *userState) Response {
	query := r.URL.Query()
	projectPath := query.Get("path")
	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	if projectPath != "" {
		project, err := project.LoadProject(c.d.overlord.WorkspaceBackend(), afero.NewOsFs(), projectPath)
		if err != nil {
			return statusNotFound("project not found: %q, %v", projectPath, err)
		}

		result := []map[string]string{
			{"project-id": project.ProjectId, "path": projectPath},
		}
		return SyncResponse(result)
	}

	// In this scenario, we will have go walk all projects in the system
	// and also make sure these are up-to-date, this is what RetrieveWorkspacesGlobal does
	// and returns a list of workspaces for every project found in the system
	projects, err := project.RetrieveWorkspacesGlobal(c.d.overlord.WorkspaceBackend(), afero.NewOsFs())
	if err != nil {
		return statusInternalError("cannot get a full list of projects: %v", err)
	}

	keys := make([]map[string]string, 0, len(projects))
	for p := range projects {
		keys = append(keys, map[string]string{
			"project-id": p.ProjectId, "path": p.Path,
		})
	}

	return SyncResponse(keys)
}
