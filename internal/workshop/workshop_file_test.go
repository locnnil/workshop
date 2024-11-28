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

	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
)

type workshopFile struct {
	fs      afero.Fs
	project *workshop.Project
}

var _ = check.Suite(&workshopFile{})

func TestWorkshop(t *testing.T) { check.TestingT(t) }

func (f *workshopFile) SetUpTest(c *check.C) {
	f.fs = afero.NewMemMapFs()

	f.project = &workshop.Project{
		Path:      c.MkDir(),
		ProjectId: "b8639dea",
	}
}

func (f *workshopFile) createWFile(c *check.C, name, yaml string, checkArgs ...interface{}) {
	path := workshop.Filepath(f.project.Path, name)

	err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
	c.Assert(err, check.IsNil, checkArgs...)

	err = os.WriteFile(path, []byte(yaml), 0644)
	c.Assert(err, check.IsNil, checkArgs...)
}

func (f *workshopFile) createSingleWFile(c *check.C, filename, yaml string, checkArgs ...interface{}) {
	err := os.MkdirAll(f.project.Path, os.ModePerm)
	c.Assert(err, check.IsNil, checkArgs...)

	err = os.WriteFile(filepath.Join(f.project.Path, filename), []byte(yaml), 0644)
	c.Assert(err, check.IsNil, checkArgs...)
}

func (f *workshopFile) TestWorkshopFileParse(c *check.C) {
	yaml := `name: xbert-gpu
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
`
	f.createWFile(c, "xbert-gpu", yaml)
	file, err := f.project.Workshop("xbert-gpu")
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
			{Name: "one", Channel: "latest/stable", Plugs: map[string]workshop.Plug{"plug": {Bind: &workshop.PlugRef{Sdk: "two", Name: "plug"}}}},
			{Name: "two", Channel: "latest/stable", Plugs: map[string]workshop.Plug{"plug": {Bind: &workshop.PlugRef{Sdk: "one", Name: "plug"}}}},
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

func (f *workshopFile) TestSingleWorkshopFile(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
`
	f.createSingleWFile(c, "workshop.yaml", yaml)
	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.IsNil)
	c.Assert(file, check.DeepEquals, &workshop.File{Name: "xbert-gpu", Base: "ubuntu@20.04"})
}

func (f *workshopFile) TestSingleWorkshopFileMissing(c *check.C) {
	file, err := f.project.SingleWorkshop()
	c.Assert(file, check.IsNil)
	message := fmt.Sprintf("default workshop definition for project %q: %v", f.project.Path, os.ErrNotExist)
	c.Assert(err, check.ErrorMatches, message)
}

func (f *workshopFile) TestSingleWorkshopFileAmbiguous(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
`
	f.createSingleWFile(c, "workshop.yaml", yaml)
	f.createSingleWFile(c, ".workshop.yaml", yaml)
	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(file, check.IsNil)
	message := fmt.Sprintf("more than one workshop definition in project %q", f.project.Path)
	c.Assert(err, check.ErrorMatches, message)
}

func (f *workshopFile) TestSingleWorkshopFileError(c *check.C) {
	path := filepath.Join(f.project.Path, "workshop.yaml")
	c.Assert(os.Mkdir(path, os.ModePerm), check.IsNil)
	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(file, check.IsNil)
	c.Assert(err, check.ErrorMatches, ".*is a directory")
}

func (f *workshopFile) TestSingleWorkshopFileHidesOthers(c *check.C) {
	singleYaml := `name: xbert-gpu
base: ubuntu@22.04
`
	olderYaml := `name: xbert-gpu
base: ubuntu@20.04
`
	hiddenYaml := `name: xbert
base: ubuntu@20.04
`
	f.createSingleWFile(c, "workshop.yaml", singleYaml)
	f.createWFile(c, "xbert-gpu", olderYaml)
	f.createWFile(c, "xbert", hiddenYaml)

	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.IsNil)
	c.Assert(file, check.DeepEquals, &workshop.File{Name: "xbert-gpu", Base: "ubuntu@22.04"})

	file, err = f.project.Workshop("xbert")
	c.Assert(file, check.IsNil)
	message := fmt.Sprintf(`single workshop in project %q is named "xbert-gpu", not "xbert"`, f.project.Path)
	c.Assert(err, check.ErrorMatches, message)
}

func (f *workshopFile) TestSingleWorkshopSymlink(c *check.C) {
	singleYaml := `name: xbert-gpu
base: ubuntu@20.04
`
	hiddenYaml := `name: xbert
base: ubuntu@20.04
`
	f.createWFile(c, "xbert-gpu", singleYaml)
	f.createWFile(c, "xbert", hiddenYaml)
	c.Assert(os.Symlink(".workshop/xbert-gpu.yaml", filepath.Join(f.project.Path, ".workshop.yaml")), check.IsNil)

	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.IsNil)
	c.Assert(file, check.DeepEquals, &workshop.File{Name: "xbert-gpu", Base: "ubuntu@20.04"})

	file, err = f.project.Workshop("xbert")
	c.Assert(file, check.IsNil)
	message := fmt.Sprintf(`single workshop in project %q is named "xbert-gpu", not "xbert"`, f.project.Path)
	c.Assert(err, check.ErrorMatches, message)
}

func (f *workshopFile) TestWorkshopNamesDifferent(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
`
	f.createWFile(c, "xbert", yaml)
	file, err := f.project.Workshop("xbert")
	c.Assert(file, check.IsNil)
	c.Assert(err, check.ErrorMatches, `"xbert-gpu" workshop file must be named "xbert-gpu.yaml" \(now: "xbert.yaml"\)`)
}

func (f *workshopFile) TestWorkshopInvalidName(c *check.C) {
	yaml := `name: 99-xbert
base: ubuntu@20.04
`
	f.createWFile(c, "99-xbert", yaml)
	file, err := f.project.Workshop("99-xbert")
	c.Assert(file, check.IsNil)
	c.Assert(err, check.ErrorMatches, `a workshop's name must: \(1\) start with a letter, \(2\) include only lowercase alphanumeric characters or underscore\(s\)`)
}

func (f *workshopFile) TestWorkshopUnsupportedBase(c *check.C) {
	yaml := `name: xbert-gpu
base: foo@20.04
`
	f.createWFile(c, "xbert-gpu", yaml)
	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(file, check.IsNil)
	c.Assert(err, check.ErrorMatches, `base "foo@20.04" not supported`)
}

func (f *workshopFile) TestWorkshopFileDuplicateSdks(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  cuda:
    channel: latest/stable
  cuda:
    channel: latest/edge
`
	f.createWFile(c, "xbert-gpu", yaml)
	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(file, check.IsNil)
	c.Assert(err, check.ErrorMatches, `"cuda" SDK must only be included once`)
}

func (f *workshopFile) TestWorkshopFileReservedNames(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  agent:
    channel: latest/stable
`
	f.createWFile(c, "xbert-gpu", yaml)
	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `"agent" is a reserved SDK name`)
	c.Assert(file, check.IsNil)
}

func (f *workshopFile) TestWorkshopUnsupportedChannel(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  cuda:
    channel: latest/foo
`
	f.createWFile(c, "xbert-gpu", yaml)
	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(file, check.IsNil)
	c.Assert(err, check.ErrorMatches, `unsupported channel "latest/foo" for "cuda" SDK`)
}

func (f *workshopFile) TestBindPlug(c *check.C) {
	yaml := `name: xbert-gpu
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
        bind: data-sdk:aux
`
	f.createWFile(c, "xbert-gpu", yaml)
	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.IsNil)
	c.Assert(file.Sdks, testutil.DeepUnsortedMatches, workshop.SdkList{
		{Name: "data-sdk", Channel: "latest/stable", Plugs: map[string]workshop.Plug{"cache": {Bind: &workshop.PlugRef{Sdk: "etl-sdk", Name: "cache"}}}},
		{Name: "etl-sdk", Channel: "latest/stable", Plugs: map[string]workshop.Plug{"data": {Bind: &workshop.PlugRef{Sdk: "data-sdk", Name: "aux"}}}},
	})
}

func (f *workshopFile) TestPlugDefinedButNotBound(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
    plugs:
      cache:
        attr1: val
  etl-sdk:
    channel: latest/stable
    plugs:
      data: 
        bind: data-sdk:aux
`
	f.createWFile(c, "xbert-gpu", yaml)
	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.IsNil)
	c.Assert(file.Sdks, testutil.DeepUnsortedMatches, workshop.SdkList{
		{Name: "data-sdk", Channel: "latest/stable", Plugs: map[string]workshop.Plug{"cache": {Attributes: map[string]interface{}{"attr1": "val"}}}},
		{Name: "etl-sdk", Channel: "latest/stable", Plugs: map[string]workshop.Plug{"data": {Bind: &workshop.PlugRef{Sdk: "data-sdk", Name: "aux"}}}},
	})
}

func (f *workshopFile) TestPlugDefinedAndBoundFails(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
  etl-sdk:
    channel: latest/stable
    plugs:
      data: 
        bind: data-sdk:aux
        attr2: val
`
	f.createWFile(c, "xbert-gpu", yaml)
	_, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `plug "data-sdk:aux" is bound and must not define other attributes`)
}

func (f *workshopFile) TestBindPlugNoSdk(c *check.C) {
	yaml := `name: xbert-gpu
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
`
	f.createWFile(c, "xbert-gpu", yaml)
	_, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `cannot bind plug "no-sdk:cache": SDK "no-sdk" not found`)
}

func (f *workshopFile) TestBindPlugToItself(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
    plugs:
      cache:
        bind: data-sdk:cache
`
	f.createWFile(c, "xbert-gpu", yaml)
	_, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `cannot bind plug "data-sdk:cache" to itself`)
}

func (f *workshopFile) TestBindPlugToBoundPlug(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  one:
    channel: latest/stable
  two:
    channel: latest/stable
    plugs:
      data:
        bind: one:data
  three:
    channel: latest/stable
    plugs:
      data:
        bind: two:data
`
	f.createWFile(c, "xbert-gpu", yaml)
	_, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `cannot bind "two:data" to "one:data": plug "two:data" is already bound`)
}

func (f *workshopFile) TestBindPlugInvalidPlugRef(c *check.C) {
	templ := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  etl-sdk:
    channel: latest/stable
    plugs:
      %s
`
	invalids := []string{
		`data:		
           bind: cache`,
		`data:
           bind: workshop/no-sdk:cache`,
		`data:
           bind: workshop:etl-sdk:cache`,
		`data:
           bind: etl-sdk`,
	}

	for _, ref := range invalids {
		f.createWFile(c, "xbert-gpu", fmt.Sprintf(templ, ref), check.Commentf(ref))
		_, err := f.project.Workshop("xbert-gpu")
		c.Assert(err, check.ErrorMatches, `.* is not a valid plug or slot reference.*`, check.Commentf(ref))
	}
}

func (f *workshopFile) TestBindToAlreadyBoundPlug(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
    plugs:
      cache:
        bind: etl-sdk:cache
      aux:
        bind: etl-sdk:data
  etl-sdk:
    channel: latest/stable
    plugs:
      cache: 
        bind: etl-sdk:data
`
	f.createWFile(c, "xbert-gpu", yaml)
	_, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `cannot bind "etl-sdk:cache" to "etl-sdk:data": plug "etl-sdk:cache" is already bound`)
}

func (f *workshopFile) TestIndirectBindToAlreadyBoundPlug(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
    plugs:
      data:
        bind: data-sdk:aux
      aux:
        bind: etl-sdk:cache
  etl-sdk:
    channel: latest/stable
    plugs:
      cache:
        bind: data-sdk:data
`
	f.createWFile(c, "xbert-gpu", yaml)
	_, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `cannot bind "data-sdk:aux" to "etl-sdk:cache": plug "data-sdk:aux" is already bound`)
}

func (f *workshopFile) TestHostSdkSlot(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  system:   
    slots:
      training-data:
        workshop-source: relative/path
`
	f.createWFile(c, "xbert-gpu", yaml)
	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.IsNil)
	c.Assert(file.Sdks, testutil.DeepUnsortedMatches, workshop.SdkList{
		{Name: sdk.System.String(), Slots: map[string]interface{}{"training-data": map[string]interface{}{"workshop-source": "relative/path"}}}})
}

func (f *workshopFile) TestWorkshopConnectionsOK(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
  etl-sdk:
    channel: latest/stable
connections:
  - plug: data-sdk:data
    slot: system:mount
  - plug: etl-sdk:data
    slot: data-sdk:data-slot
`
	f.createWFile(c, "xbert-gpu", yaml)
	file, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.IsNil)
	c.Assert(file.Connections, testutil.DeepUnsortedMatches, []workshop.Connection{
		{PlugRef: workshop.PlugRef{Sdk: "data-sdk", Name: "data"}, SlotRef: workshop.SlotRef{Sdk: sdk.System.String(), Name: "mount"}},
		{PlugRef: workshop.PlugRef{Sdk: "etl-sdk", Name: "data"}, SlotRef: workshop.SlotRef{Sdk: "data-sdk", Name: "data-slot"}},
	})
}

func (f *workshopFile) TestWorkshopConnectionsInvalidRefs(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
  etl-sdk:
    channel: latest/stable
connections:
  - plug: data-sdk
    slot: system:mount
  - plug: etl-sdk:data
    slot: data-sdk:data-slot
`
	f.createWFile(c, "xbert-gpu", yaml)
	_, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `"data-sdk" is not a valid plug or slot reference \(use <sdk>:<plug or slot>\)`)
}

func (f *workshopFile) TestWorkshopConnectionsSlotSdkNotInTheList(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
  etl-sdk:
    channel: latest/stable
connections:
  - plug: data-sdk:data
    slot: lost-sdk:mount
`
	f.createWFile(c, "xbert-gpu", yaml)
	_, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `cannot connect plug "data-sdk:data" to slot "lost-sdk:mount": workshop "xbert-gpu" has no SDK named "lost-sdk"`)
}

func (f *workshopFile) TestWorkshopConnectionsPlugSdkNotInTheList(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
  etl-sdk:
    channel: latest/stable
connections:
  - plug: lost-sdk:data
    slot: data-sdk:mount
`
	f.createWFile(c, "xbert-gpu", yaml)
	_, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `cannot connect plug "lost-sdk:data" to slot "data-sdk:mount": workshop "xbert-gpu" has no SDK named "lost-sdk"`)
}

func (f *workshopFile) TestWorkshopConnectionsImplicitHostSdkPlugSlot(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
  etl-sdk:
    channel: latest/stable
connections:
  - plug: system:data
    slot: system:mount
`
	f.createWFile(c, "xbert-gpu", yaml)
	_, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.IsNil)
}

func (f *workshopFile) TestWorkshopConnectionsBoundPlugCannotBeConnected(c *check.C) {
	yaml := `name: xbert-gpu
base: ubuntu@20.04
sdks:
  data-sdk:
    channel: latest/stable
    plugs:
      data: 
        bind: etl-sdk:data
  etl-sdk:
    channel: latest/stable
connections:
  - plug: data-sdk:data
    slot: system:mount
  - plug: etl-sdk:data
    slot: data-sdk:data-slot
`
	f.createWFile(c, "xbert-gpu", yaml)
	_, err := f.project.Workshop("xbert-gpu")
	c.Assert(err, check.ErrorMatches, `cannot connect plug "data-sdk:data" to slot "system:mount": plug is bound`)
}
