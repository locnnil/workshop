package workshopbackend_test

import (
	"os"
	"path/filepath"

	"github.com/canonical/workshop/internal/workshopbackend"
	"github.com/spf13/afero"
	"golang.org/x/exp/slices"
	"gopkg.in/check.v1"
)

type F struct {
	fs afero.Fs
}

var _ = check.Suite(&F{})

func (f *F) SetUpTest(c *check.C) {
	f.fs = afero.NewMemMapFs()
}

func (f *F) TestWorkshopFileParse(c *check.C) {
	buf := []byte(`name: xbert-gpu
base: ubuntu@20.04
sdks:
  huggingface:
    channel: latest/stable
  cuda:
    channel: latest/edge
  zookeeper:
    channel: latest/candidate
  automotive:
    channel: latest/beta
`)
	dir := c.MkDir()
	os.WriteFile(filepath.Join(dir, ".workshop.xbert-gpu.yaml"), buf, 0644)
	file, err := workshopbackend.ReadWorkshop(workshopbackend.WorkshopFilePath(dir, "xbert-gpu"))
	c.Assert(err, check.Equals, nil)
	c.Assert(file.Name, check.Equals, "xbert-gpu")
	c.Assert(file.Base, check.Equals, "ubuntu@20.04")
	c.Assert(slices.IsSortedFunc(file.Sdks, func(a, b workshopbackend.SdkRecord) bool {
		return a.Name < b.Name
	}), check.Equals, true)
	c.Assert(file.Sdks[0].Name, check.Equals, "automotive")
	c.Assert(file.Sdks[0].Channel, check.Equals, "latest/beta")
	c.Assert(file.Sdks[1].Name, check.Equals, "cuda")
	c.Assert(file.Sdks[1].Channel, check.Equals, "latest/edge")
	c.Assert(file.Sdks[2].Name, check.Equals, "huggingface")
	c.Assert(file.Sdks[2].Channel, check.Equals, "latest/stable")
	c.Assert(file.Sdks[3].Name, check.Equals, "zookeeper")
	c.Assert(file.Sdks[3].Channel, check.Equals, "latest/candidate")
}
