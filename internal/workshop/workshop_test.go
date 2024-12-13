package workshop_test

import (
	"context"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	"github.com/canonical/workshop/internal/workshop/fakebackend"
)

type workshopSuite struct {
	bend *fakebackend.FakeWorkshopBackend
	ctx  context.Context
	p    *workshop.Project

	restoreUserLookup func()
}

var _ = check.Suite(&workshopSuite{})

var workshopyaml = []byte(`name: test-workshop
base: ubuntu@22.04`)

func (f *workshopSuite) SetUpTest(c *check.C) {
	var err error
	f.bend, err = fakebackend.New(c.MkDir())
	c.Assert(err, check.IsNil)

	userhomedir := c.MkDir()
	f.restoreUserLookup = testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{
			Name:     name,
			Username: name,
			Uid:      "1000",
			Gid:      "1000",
			HomeDir:  userhomedir,
		}
		return u, nil
	}, &workshop.LookupUsername)

	ctx := context.WithValue(context.Background(), workshop.ContextUser, "testuser")

	f.p, _, err = f.bend.CreateOrLoadProject(ctx, c.MkDir())
	c.Assert(err, check.IsNil)

	f.ctx = createTestContext("testuser", f.p.ProjectId)
}

func (f *workshopSuite) TearDownTest(c *check.C) {
	f.restoreUserLookup()
}

func createTestContext(username, projectId string) context.Context {
	ctx := context.WithValue(context.Background(), workshop.ContextUser, username)
	ctx = context.WithValue(ctx, workshop.ContextProjectId, projectId)
	return ctx
}

func writeFile(c *check.C, path string, content string) {
	c.Assert(os.MkdirAll(filepath.Dir(path), 0755), check.IsNil)
	c.Assert(os.WriteFile(path, []byte(content), 0644), check.IsNil)
}

func (f *workshopSuite) TestInstallLocalSdkMetaOnlyOK(c *check.C) {
	wpath := filepath.Join(f.p.Path, "workshop.yaml")
	writeFile(c, wpath, string(workshopyaml))
	file, err := workshop.ReadWorkshop(wpath)
	c.Assert(err, check.IsNil)

	err = f.bend.LaunchWorkshop(f.ctx, file)
	c.Assert(err, check.IsNil)

	w, err := f.bend.Workshop(f.ctx, "test-workshop")
	c.Assert(err, check.IsNil)

	localdir := c.MkDir()
	writeFile(c, filepath.Join(localdir, "meta/sdk.yaml"), `name: local
base: ubuntu@22.04`)
	localsdkFs := os.DirFS(localdir)

	err = w.InstallLocalSdk(f.ctx, "local", "x1", localsdkFs)
	c.Assert(err, check.IsNil)

	wfs, err := f.bend.WorkshopFs(f.ctx, "test-workshop")
	c.Assert(err, check.IsNil)

	_, err = wfs.Stat("/var/lib/workshop/sdk/local/x1/meta/sdk.yaml")
	c.Assert(err, check.IsNil)
}

func (f *workshopSuite) TestInstallLocalSdkNoMetaFails(c *check.C) {
	wpath := filepath.Join(f.p.Path, "workshop.yaml")
	writeFile(c, wpath, string(workshopyaml))
	file, err := workshop.ReadWorkshop(wpath)
	c.Assert(err, check.IsNil)

	err = f.bend.LaunchWorkshop(f.ctx, file)
	c.Assert(err, check.IsNil)

	w, err := f.bend.Workshop(f.ctx, "test-workshop")
	c.Assert(err, check.IsNil)

	localdir := c.MkDir()
	localsdkFs := os.DirFS(localdir)

	err = w.InstallLocalSdk(f.ctx, "local", "x1", localsdkFs)
	c.Assert(err, check.ErrorMatches, `open meta/sdk.yaml: no such file or directory`)

	wfs, err := f.bend.WorkshopFs(f.ctx, "test-workshop")
	c.Assert(err, check.IsNil)

	_, err = wfs.Stat("/var/lib/workshop/sdk/local/x1/meta")
	c.Assert(osutil.IsDirNotExist(err), check.Equals, true)
}

func (f *workshopSuite) TestInstallLocalSdkWithHooksOK(c *check.C) {
	wpath := filepath.Join(f.p.Path, "workshop.yaml")
	writeFile(c, wpath, string(workshopyaml))
	file, err := workshop.ReadWorkshop(wpath)
	c.Assert(err, check.IsNil)

	err = f.bend.LaunchWorkshop(f.ctx, file)
	c.Assert(err, check.IsNil)

	w, err := f.bend.Workshop(f.ctx, "test-workshop")
	c.Assert(err, check.IsNil)

	localdir := c.MkDir()
	writeFile(c, filepath.Join(localdir, "meta/sdk.yaml"), `name: local	
base: ubuntu@22.04`)
	writeFile(c, filepath.Join(localdir, "hooks/setup-base"), "")
	localsdkFs := os.DirFS(localdir)

	err = w.InstallLocalSdk(f.ctx, "local", "x1", localsdkFs)
	c.Assert(err, check.IsNil)

	wfs, err := f.bend.WorkshopFs(f.ctx, "test-workshop")
	c.Assert(err, check.IsNil)

	_, err = wfs.Stat("/var/lib/workshop/sdk/local/x1/meta/sdk.yaml")
	c.Assert(err, check.IsNil)

	info, err := wfs.Stat("/var/lib/workshop/sdk/local/x1/sdk/hooks/setup-base")
	c.Assert(err, check.IsNil)
	c.Check(info.Mode().Perm(), check.Equals, fs.FileMode(0755))
}
