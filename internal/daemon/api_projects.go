package daemon

import "net/http"

func v1Projects(c *Command, r *http.Request, _ *userState) Response {
	query := r.URL.Query()
	projectPath := query.Get("project")
	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	key, err := c.d.overlord.ProjectManager().LoadProject(projectPath)
	if err != nil {
		return statusNotFound("project not found: %q, %v", projectPath, err)
	}

	result := map[string]interface{}{
		"project-id": key.ProjectId,
	}
	return SyncResponse(result)
}
