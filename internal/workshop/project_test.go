package workshop_test

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
)

type projectSuite struct {
}

var _ = check.Suite(&projectSuite{})

var w = `name: %s
base: ubuntu@22.04
`

var wb = `name: wb
base: ubuntu@22.04
connections:
  - plug:
	- system:plug
`

func createWorkshop(dir, name string) error {
	return os.WriteFile(filepath.Join(dir, workshop.Filename(name)), []byte(fmt.Sprintf(w, name)), 0644)
}

func (f *workshopFile) TestSomeWorkshopFilesBroken(c *check.C) {
	d := c.MkDir()
	p := &workshop.Project{Path: d, ProjectId: "42424242"}

	w := filepath.Join(d, workshop.Directory)
	c.Assert(os.MkdirAll(w, os.ModePerm), check.IsNil)

	c.Assert(createWorkshop(w, "w1"), check.IsNil)
	c.Assert(createWorkshop(w, "w2"), check.IsNil)
	c.Assert(createWorkshop(w, "-"), check.IsNil)
	c.Assert(os.MkdirAll(filepath.Join(w, "workshop.test-dir.yaml"), 0755), check.IsNil)
	// broken workshop
	c.Assert(os.WriteFile(filepath.Join(w, "workshop.wb.yaml"), []byte(wb), 0644), check.IsNil)
	// no match with the filename pattern
	c.Assert(os.WriteFile(filepath.Join(w, ".workshop.test-dir.yaml"), []byte{}, 0644), check.IsNil)
	fls, err := p.ReadWorkshops()
	c.Assert(err, check.IsNil)
	c.Assert(fls, check.HasLen, 4)
	c.Assert(fls, testutil.DeepUnsortedMatches, []string{"w1", "w2", "wb", "-"})
}

func (f *workshopFile) TestWorkshopNamesDifferent(c *check.C) {
	d := c.MkDir()
	p := &workshop.Project{Path: d, ProjectId: "42424242"}
	buf := []byte(`name: xbert-gpu
base: ubuntu@20.04
`)
	c.Assert(os.WriteFile(filepath.Join(d, ".workshop.xbert.yaml"), []byte(buf), 0644), check.IsNil)

	file, err := p.Workshop("xbert")
	c.Assert(file, check.IsNil)
	c.Assert(err, check.ErrorMatches, `"xbert-gpu" workshop file must be named as ".workshop.xbert-gpu.yaml" \(now: .workshop.xbert.yaml\)`)
}
