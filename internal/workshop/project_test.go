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
	c.Assert(os.MkdirAll(filepath.Join(w, "test-dir.yaml"), 0755), check.IsNil)
	// broken workshop
	c.Assert(os.WriteFile(filepath.Join(w, "wb.yaml"), []byte(wb), 0644), check.IsNil)
	// no match with the filename pattern
	c.Assert(os.WriteFile(filepath.Join(w, "test-file.yml"), []byte{}, 0644), check.IsNil)
	fls, err := p.ReadWorkshops()
	c.Assert(err, check.IsNil)
	c.Assert(fls, check.HasLen, 4)
	c.Assert(fls, testutil.DeepUnsortedMatches, []string{"w1", "w2", "wb", "-"})
}
