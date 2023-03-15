package workspace

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
)

type workspaceFile struct {
	Name string          `yaml:"name"`
	Base string          `yaml:"base"`
	Sdks map[string]*Sdk `yaml:"sdks"`
}

type Sdk struct {
	Channel string `yaml:"channel"`
}

func Launch(st *state.State, project *Project, file *workspaceFile) (*state.TaskSet, error) {
	create := st.NewTask("create-workspace-base", fmt.Sprintf("Create workspace %q base", file.Name))
	create.Set("workspace-base", file.Base)

	addProjectDir := st.NewTask("add-device", fmt.Sprintf("Mount project directory %q ", project.ProjectId()))
	addProjectDir.WaitFor(create)

	start := st.NewTask("set-workspace-state", fmt.Sprintf("Start workspace %q", project.ProjectId()))
	start.Set("workspace-state", "start")
	start.WaitFor(addProjectDir)

	set := state.NewTaskSet(create, addProjectDir, start)

	return set, nil
}
