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
	return SyncResponse([]map[string]string{})
}
