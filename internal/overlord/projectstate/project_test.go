package projectstate

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/spf13/afero"
	"golang.org/x/exp/slices"

	. "gopkg.in/check.v1"
)

type P struct {
	Fs      afero.Fs
	Backend workspacebackend.WorkspaceBackend
	ctx     context.Context
}

var _ = Suite(&P{})

func Test(t *testing.T) { TestingT(t) }

func (p *P) SetUpTest(c *C) {
	p.Fs = afero.NewMemMapFs()
	p.Backend = workspacebackend.NewFakeWorkspaceBackend()
	p.Fs.MkdirAll(util.DataDir, 0755)
	p.Fs.MkdirAll(util.SdksDir, 0755)
	p.ctx = context.WithValue(context.TODO(), workspacebackend.ContextProjectId, "projectId")
	rand.Seed(1)
}

func (p *P) TestEnumWorkspacesInACWD(c *C) {
	afero.WriteFile(p.Fs, ".workspace.project1.yaml", []byte(""), 0644)
	afero.WriteFile(p.Fs, ".workspace.project2.yaml", []byte(""), 0644)
	afero.WriteFile(p.Fs, "workspace.project3.yaml", []byte(""), 0644)
	p.Fs.Mkdir(".workspace.project2dir.yaml", 0755)
	afero.WriteFile(p.Fs, ".workspace.yaml", []byte(""), 0644)
	afero.WriteFile(p.Fs, ".workspace.lock", []byte(""), 0644)

	project := &Project{fs: p.Fs}
	ws, err := project.EnumWorkspaceFiles()

	c.Check(ws, HasLen, 2)

	c.Check(ws[0].Name, Equals, "project1")
	c.Check(ws[1].Name, Equals, "project2")
	c.Check(err, Equals, nil)
}

func (s *P) TestNewProject(c *C) {
	project, err := NewProject(s.Backend, s.Fs, "/")
	c.Check(project.ProjectId(), Equals, "52fdfc07")
	c.Check(err, Equals, nil)

	c.Check(project.ProjectDirectory(), Equals, "/")

	_, err = NewProject(s.Backend, s.Fs, "/doesnotexist")
	c.Check(errors.Is(err, afero.ErrFileNotFound), Equals, true)
}

func (s *P) TestLoadProject(c *C) {
	fs := afero.NewOsFs()
	fs.MkdirAll("/tmp/experiments", 0755)
	defer fs.RemoveAll("/tmp/experiments")

	/* No relative paths are allowed */
	_, err := LoadProject(nil, fs, "../tmp/experiments")
	c.Check(errors.Is(err, util.ErrNoRelativePathsAllowed), Equals, true)

	/* Could not read the project directory */
	_, err = LoadProject(nil, fs, "/invalid&")
	c.Check(err, NotNil)

	/* Project exists, no workspace instances */
	afero.WriteFile(s.Fs, "/.workspace.lock", []byte("projectId"), 0644)
	project, err := LoadProject(s.Backend, s.Fs, "/")
	c.Check(project.ProjectDirectory(), Equals, "/")
	c.Check(project.ProjectId(), Equals, "projectId")
	c.Check(err, IsNil)

	/* Project exists, some workspace instances running */
	s.Backend.LaunchWorkspace(s.ctx, "ws", "ubuntu@20.04")
	project, err = LoadProject(s.Backend, s.Fs, "/")
	c.Check(project.ProjectDirectory(), Equals, "/")
	c.Check(err, IsNil)
}

func (s *P) TestEnumWorkspacesNoFilesNoInstances(c *C) {
	project := Project{fs: s.Fs, backend: s.Backend, path: "/"}

	result, err := project.RetrieveWorkspaces()

	c.Check(result, HasLen, 0)
	c.Check(err, IsNil)
}

func (s *P) TestEnumFilesErrorReadingProjectDirectory(c *C) {
	project := Project{fs: s.Fs, backend: s.Backend, path: "/"}
	s.Fs.RemoveAll("/")

	_, err := project.EnumWorkspaceFiles()

	c.Check(err, NotNil)
}

func (s *P) TestEnumWorkspacesFilesOnly(c *C) {
	project := Project{fs: s.Fs, backend: s.Backend, path: "/"}
	afero.WriteFile(s.Fs, ".workspace.project1.yaml", []byte(""), 0644)
	afero.WriteFile(s.Fs, ".workspace.lock", []byte(""), 0644)

	result, err := project.RetrieveWorkspaces()
	c.Check(err, IsNil)
	c.Check(result, HasLen, 1)
	c.Check(result[0].State(), Equals, util.Inactive)
	c.Check(result[0].Reason(), Equals, util.None)
}

func (s *P) TestEnumWorkspacesInstancesOnly(c *C) {
	s.Backend.LaunchWorkspace(s.ctx, "instance1", "ubuntu@20.04")
	project := Project{fs: s.Fs, backend: s.Backend, path: "/", projectId: "projectId"}

	result, err := project.RetrieveWorkspaces()
	c.Check(err, IsNil)
	c.Assert(result, HasLen, 1)
	/* the workspace does not have a corresponding file, hence, an error state */
	c.Check(result[0].Name, Equals, "instance1")
	c.Check(result[0].State(), Equals, util.Error)
	c.Check(result[0].Reason(), Equals, util.MissingFile)
}

func (s *P) TestEnumWorkspacesSomeOrphanedInstances(c *C) {
	s.Backend.LaunchWorkspace(s.ctx, "instance1", "ubuntu@20.04")
	s.Backend.LaunchWorkspace(s.ctx, "project1", "ubuntu@20.04")

	project := Project{fs: s.Fs, backend: s.Backend, path: "/", projectId: "projectId"}
	afero.WriteFile(s.Fs, ".workspace.project1.yaml", []byte(""), 0644)

	result, err := project.RetrieveWorkspaces()
	// Make sure the order is always predictable
	slices.SortFunc(result, func(i, j *workspacebackend.WorkspaceProps) bool { return i.Name < j.Name })
	c.Check(err, IsNil)
	c.Assert(result, HasLen, 2)

	c.Check(result[0].Name, Equals, "instance1")
	c.Check(result[0].State(), Equals, util.Error)
	c.Check(result[0].Reason(), Equals, util.MissingFile)

	c.Check(result[1].Name, Equals, "project1")
	c.Check(result[1].State(), Equals, util.Ready)
	c.Check(result[1].Reason(), Equals, util.None)
}

func (s *P) TestReadProject(c *C) {
	project := Project{fs: s.Fs, path: "/project"}

	err := project.ReadProject()
	c.Check(err, NotNil)

	afero.WriteFile(s.Fs, "/.workspace.lock", []byte("23451S"), 0644)

	project = Project{fs: s.Fs, path: "/"}
	err = project.ReadProject()
	c.Check(err, IsNil)
	c.Check(project.ProjectId(), Equals, "23451S")
}
