package workshop_test

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"golang.org/x/exp/slices"
	"gopkg.in/check.v1"
	"gopkg.in/yaml.v3"

	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
)

type workshopFile struct {
	fs afero.Fs
}

var _ = check.Suite(&workshopFile{})

func TestWorkshop(t *testing.T) { check.TestingT(t) }

func (f *workshopFile) SetUpTest(c *check.C) {
	f.fs = afero.NewMemMapFs()
}

func workshopFilePath(dir, name string) string {
	return filepath.Join(dir, fmt.Sprintf(".workshop.%s.yaml", name))
}

func (f *workshopFile) TestWorkshopFileParse(c *check.C) {
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
	c.Assert(os.WriteFile(filepath.Join(dir, ".workshop.xbert-gpu.yaml"), buf, 0644), check.IsNil)
	file, err := workshop.ReadWorkshop(workshopFilePath(dir, "xbert-gpu"))
	c.Assert(err, check.Equals, nil)
	c.Assert(file.Name, check.Equals, "xbert-gpu")
	c.Assert(file.Base, check.Equals, "ubuntu@20.04")
	c.Assert(slices.IsSortedFunc(file.Sdks, func(a, b workshop.SdkRecord) int {
		return cmp.Compare(a.Name, b.Name)
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

func (f *workshopFile) TestWorkshopFileSave(c *check.C) {
	fl := &workshop.File{
		Name: "test-workshop",
		Base: "ubuntu@22.04",
		Sdks: []workshop.SdkRecord{
			{Name: "one", Channel: "latest/stable", Plugs: map[string]workshop.Plug{"plug": {Bind: workshop.Bind{Sdk: "two", Plug: "plug"}}}},
			{Name: "two", Channel: "latest/stable", Plugs: map[string]workshop.Plug{"plug": {Bind: workshop.Bind{Sdk: "one", Plug: "plug"}}}},
		},
	}
	out, err := yaml.Marshal(fl)
	c.Assert(err, check.IsNil)
	c.Assert(string(out), check.Equals, `name: test-workshop
base: ubuntu@22.04
sdks:
    one:
        channel: latest/stable
        plugs:
            plug:
                bind: two:plug
    two:
        channel: latest/stable
        plugs:
            plug:
                bind: one:plug
`)
}

func (f *workshopFile) TestWorkshopFileDuplicateSdks(c *check.C) {
	buf := []byte(`name: xbert-gpu
base: ubuntu@20.04
sdks:
  cuda:
    channel: latest/stable
  cuda:
    channel: latest/edge
`)
	dir := c.MkDir()
	c.Assert(os.WriteFile(filepath.Join(dir, ".workshop.xbert-gpu.yaml"), buf, 0644), check.IsNil)
	file, err := workshop.ReadWorkshop(workshopFilePath(dir, "xbert-gpu"))
	c.Assert(file, check.IsNil)
	c.Assert(err, check.ErrorMatches, `"cuda" SDK must only be included once`)
}

func (f *workshopFile) TestWorkshopFileReservedNames(c *check.C) {
	buf := []byte(`name: xbert-gpu
base: ubuntu@20.04
sdks:
  agent:
    channel: latest/stable
`)
	dir := c.MkDir()
	c.Assert(os.WriteFile(filepath.Join(dir, ".workshop.xbert-gpu.yaml"), buf, 0644), check.IsNil)
	file, err := workshop.ReadWorkshop(workshopFilePath(dir, "xbert-gpu"))
	c.Assert(err, check.ErrorMatches, `"agent" is a reserved SDK name`)
	c.Assert(file, check.IsNil)
}

func (f *workshopFile) TestBindPlug(c *check.C) {
	buf := []byte(`name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
    plugs:
      cache:
        bind: etl-sdk:cache
  etl-sdk:
    channel: latest/stable
    plugs:
      data: 
        bind: data-sdk:cache
`)
	dir := c.MkDir()
	p := workshop.Project{Path: dir, ProjectId: "42424242"}
	c.Assert(os.WriteFile(filepath.Join(dir, ".workshop.xbert-gpu.yaml"), buf, 0644), check.IsNil)
	file, err := p.Workshop("xbert-gpu")
	c.Assert(err, check.IsNil)
	c.Assert(file.Sdks, testutil.DeepUnsortedMatches, workshop.SdkList{
		workshop.SdkRecord{Name: "data-sdk", Channel: "latest/stable", Plugs: map[string]workshop.Plug{"cache": {Bind: workshop.Bind{Sdk: "etl-sdk", Plug: "cache"}}}},
		workshop.SdkRecord{Name: "etl-sdk", Channel: "latest/stable", Plugs: map[string]workshop.Plug{"data": {Bind: workshop.Bind{Sdk: "data-sdk", Plug: "cache"}}}},
	})
}

func (f *workshopFile) TestBindPlugNoSdk(c *check.C) {
	buf := []byte(`name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
    plugs:
      cache:
        bind: no-sdk:cache
  etl-sdk:
    channel: latest/stable
    plugs:
      data: 
        bind: data-sdk:cache
`)
	dir := c.MkDir()
	p := workshop.Project{Path: dir, ProjectId: "42424242"}
	c.Assert(os.WriteFile(filepath.Join(dir, ".workshop.xbert-gpu.yaml"), buf, 0644), check.IsNil)
	_, err := p.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `"no-sdk:cache" tries to bind to a plug from a non-existing SDK`)
}

func (f *workshopFile) TestBindPlugIncorrectSdkName(c *check.C) {
	buf := []byte(`name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
    plugs:
      cache:
        bind: workshop/no-sdk:cache
  etl-sdk:
    channel: latest/stable
    plugs:
      data: 
        bind: data-sdk:cache
`)
	dir := c.MkDir()
	p := workshop.Project{Path: dir, ProjectId: "42424242"}
	c.Assert(os.WriteFile(filepath.Join(dir, ".workshop.xbert-gpu.yaml"), buf, 0644), check.IsNil)
	_, err := p.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `"workshop/no-sdk" isn't a valid SDK name`)
}

func (f *workshopFile) TestBindPlugIncorrectPlugRef(c *check.C) {
	buf := []byte(`name: xbert-gpu
base: ubuntu@20.04
sdks:
  etl-sdk:
    channel: latest/stable
    plugs:
      data: 
        bind: cache
`)
	dir := c.MkDir()
	p := workshop.Project{Path: dir, ProjectId: "42424242"}
	c.Assert(os.WriteFile(filepath.Join(dir, ".workshop.xbert-gpu.yaml"), buf, 0644), check.IsNil)
	_, err := p.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `incorrect bind plug reference: "cache" \(use <sdk>:<plug>\)`)
}
