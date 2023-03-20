package projectstate

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
)

func LoadOrCreate(st *state.State, projectDir string) (*state.Task, error) {
	load := st.NewTask("load-or-create-project", fmt.Sprintf("Load project from %q", projectDir))
	load.Set("project-directory", projectDir)
	return load, nil
}
