package daemon

import (
	"net/http"
	"os/user"
	"strconv"

	"github.com/canonical/workspace/internal/project"
	"github.com/canonical/workspace/internal/workspacebackend"

	"github.com/spf13/afero"
	"golang.org/x/net/context"
)

var LookupUsername = user.LookupId

func v1Projects(c *Command, r *http.Request, _ *userState) Response {
	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	_, uid, _, err := ucrednetGet(r.RemoteAddr)
	if err != nil {
		return statusInternalError("cannot get an associated uid: %v", err)
	}

	username, err := LookupUsername(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return statusInternalError("cannot get an associated user name: %v", err)
	}

	userCtx := context.WithValue(r.Context(), workspacebackend.ContextUser, username.Username)

	// In this scenario, we will have go walk all projects in the system
	// and also make sure these are up-to-date, this is what RetrieveWorkspacesGlobal does
	// and returns a list of workspaces for every project found in the system
	projects, err := project.RetrieveAllProjects(userCtx, c.d.overlord.WorkspaceBackend(), afero.NewOsFs())
	if err != nil {
		return statusInternalError("cannot get a full list of projects: %v", err)
	}

	return SyncResponse(projects)
}

func v1GetProjectWorkspace(c *Command, r *http.Request, _ *userState) Response {
	return SyncResponse([]string{})
}
