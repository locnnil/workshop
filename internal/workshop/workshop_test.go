package workshop_test

import (
	"os"
	"path/filepath"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

type workshopSuite struct {
	project workshop.Project
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
	f.project = workshop.Project{
		Path:      c.MkDir(),
		ProjectId: "b8639dea",
	}
}

func writeFile(c *check.C, path string, content string) {
	c.Assert(os.MkdirAll(filepath.Dir(path), 0755), check.IsNil)
	c.Assert(os.WriteFile(path, []byte(content), 0644), check.IsNil)
}

func (f *workshopSuite) TestSdkSetupsByInstallOrder(c *check.C) {
	wpath := filepath.Join(f.project.Path, "workshop.yaml")
	writeFile(c, wpath, string(workshopyaml))
	file, err := workshop.ReadWorkshop(wpath)
	c.Assert(err, check.IsNil)

	w := workshop.Workshop{File: file, Name: "test-workshop"}
	w.Sdks = map[string]sdk.Setup{
		"test-sdk-1": {Name: "test-sdk-1", Revision: sdk.R(1), Channel: "latest/stable"},
		"test-sdk-2": {Name: "test-sdk-2", Revision: sdk.R(1), Channel: "latest/edge"},
		"system":     {Name: "system", Source: sdk.SystemSource, Revision: sdk.R(1)},
		"sketch":     {Name: "sketch", Source: sdk.SketchSource, Revision: sdk.R(-3)},
	}

	sdks := w.SdksByInstallOrder()
	c.Assert(sdks, check.DeepEquals, []sdk.Setup{
		{Name: "system", Source: sdk.SystemSource, Revision: sdk.R(1)},
		{Name: "test-sdk-1", Revision: sdk.R(1), Channel: "latest/stable"},
		{Name: "test-sdk-2", Revision: sdk.R(1), Channel: "latest/edge"},
		{Name: "sketch", Source: sdk.SketchSource, Revision: sdk.R(-3)},
	})
}
