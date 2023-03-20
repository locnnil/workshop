package workspace_test

import (
	"testing"

	workspace "github.com/canonical/workspace/internal/overlord/workspacestate"
	"github.com/canonical/workspace/internal/server"
	"github.com/spf13/afero"
	. "gopkg.in/check.v1"
)

type F struct {
	fs      afero.Fs
	project *workspace.Project
}

var _ = Suite(&F{})

func TestFile(t *testing.T) { TestingT(t) }

func (f *F) SetUpTest(c *C) {
	f.fs = afero.NewMemMapFs()
	server := server.WorkspaceServer(nil)
	f.project, _ = workspace.NewProject(server, f.fs, "/")
}

func (f *F) TestWorkspaceFileParseNormal(c *C) {
	buf := []byte(`name: xbert-gpu
base: ubuntu@20.04
sdks:
  huggingface:
    channel: latest/stable
  cuda:
    channel: latest/stable
`)
	afero.WriteFile(f.fs, "/.workspace.xbert-gpu.yaml", buf, 0644)
	file, err := workspace.ReadWorkspace(f.project, "xbert-gpu")
	c.Assert(err, Equals, nil)
	c.Assert(file.Name, Equals, "xbert-gpu")
	c.Assert(file.Base, Equals, "ubuntu@20.04")
	c.Assert(file.Sdks[0].Name, Equals, "huggingface")
	c.Assert(file.Sdks[0].Channel, Equals, "latest/stable")
	c.Assert(file.Sdks[1].Name, Equals, "cuda")
	c.Assert(file.Sdks[1].Channel, Equals, "latest/stable")

}
