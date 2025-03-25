package workshop_test

import (
	"context"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
	"github.com/canonical/workshop/internal/workshop/fakebackend"
)

type workshopSuite struct {
	bend *fakebackend.FakeWorkshopBackend
	ctx  context.Context
	p    workshop.Project

	restoreUserLookup func()
}

var _ = check.Suite(&workshopSuite{})

var workshopyaml = []byte(`name: test-workshop
base: ubuntu@22.04
sdks:
  - name: test-sdk-1
    channel: latest/stable
  - name: test-sdk-2
    channel: latest/stable
  - name: system
`)

func (f *workshopSuite) SetUpTest(c *check.C) {
	var err error
	f.bend, err = fakebackend.New(c.MkDir())
	c.Assert(err, check.IsNil)

	f.restoreUserLookup = osutil.FakeUserLookup(func(name string) (*user.User, error) {
		return &user.User{HomeDir: c.MkDir()}, nil
	})

	ctx := context.WithValue(context.Background(), workshop.ContextUser, "testuser")

	p, _, err := f.bend.CreateOrLoadProject(ctx, c.MkDir())
	c.Assert(err, check.IsNil)
	f.p = *p

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

	err = f.bend.LaunchOrRebuildWorkshop(f.ctx, file)
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

	err = f.bend.LaunchOrRebuildWorkshop(f.ctx, file)
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

	err = f.bend.LaunchOrRebuildWorkshop(f.ctx, file)
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

func (f *workshopSuite) TestSdkSetupsByInstallOrder(c *check.C) {
	wpath := filepath.Join(f.p.Path, "workshop.yaml")
	writeFile(c, wpath, string(workshopyaml))
	file, err := workshop.ReadWorkshop(wpath)
	c.Assert(err, check.IsNil)

	w := workshop.Workshop{File: file, Name: "test-workshop"}
	w.Sdks = map[string]sdk.Setup{
		"test-sdk-1": {Name: "test-sdk-1", Revision: sdk.R(1), Channel: "latest/stable"},
		"test-sdk-2": {Name: "test-sdk-2", Revision: sdk.R(1), Channel: "latest/edge"},
		"system":     {Name: "system", Revision: sdk.R(-1)},
		"sketch":     {Name: "sketch", Revision: sdk.R(-3)},
	}

	sdks := w.SdksByInstallOrder()
	c.Assert(sdks, check.DeepEquals, []sdk.Setup{
		{Name: "system", Revision: sdk.R(-1)},
		{Name: "test-sdk-1", Revision: sdk.R(1), Channel: "latest/stable"},
		{Name: "test-sdk-2", Revision: sdk.R(1), Channel: "latest/edge"},
		{Name: "sketch", Revision: sdk.R(-3)},
	})
}
