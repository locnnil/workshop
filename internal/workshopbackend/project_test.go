package workshopbackend_test

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/workshopbackend"
)

type projectSuite struct {
}

var _ = check.Suite(&projectSuite{})

var w = `name: %s
base: ubuntu@22.04
`

func createWorkshop(dir, name string) error {
	return os.WriteFile(filepath.Join(dir, fmt.Sprintf(".workshop.%s.yaml", name)), []byte(fmt.Sprintf(w, name)), 0644)
}

func (f *workshopFile) TestSomeWorkshopFilesBroken(c *check.C) {
	d := c.MkDir()
	p := &workshopbackend.Project{Path: d, ProjectId: "42424242"}
	c.Assert(createWorkshop(d, "w1"), check.IsNil)
	c.Assert(createWorkshop(d, "w2"), check.IsNil)
	c.Assert(createWorkshop(d, "-"), check.IsNil)
	fls, err := p.ReadWorkshops()
	c.Assert(err, check.IsNil)
	c.Assert(fls, check.HasLen, 2)
}
