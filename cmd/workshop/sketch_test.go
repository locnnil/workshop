package main

import (
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
)

type workshopSketch struct {
	BaseWorkshopSuite
	prjDir string
	prjId  string
	user   *user.User
}

var _ = check.Suite(&workshopSketch{})

var simpleSketchMeta = `name: sketch
base: ubuntu@22.04
`

func (m *workshopSketch) SetUpTest(c *check.C) {
	m.BaseWorkshopSuite.SetUpTest(c)

	m.prjDir = c.MkDir()
	m.prjId = "42424242"
	var err error
	m.user, err = osutil.UserMaybeSudoUser()
	c.Assert(err, check.IsNil)
}

func (m *workshopSketch) TearDownTest(c *check.C) {
	sketch := sdk.ProjectSketchSdkDir(m.user.HomeDir, m.prjId)
	err := os.RemoveAll(sketch)
	c.Assert(err, check.IsNil)
}

func (m *workshopSketch) mockMinimalSketchSdk(c *check.C, current bool, meta []byte) (metapath string, hookspath string) {
	var rootdir string
	if current {
		rootdir = sdk.WorkshopSketchSdkCurrent(m.user.HomeDir, m.prjId, "ws")
	} else {
		rootdir = sdk.WorkshopSketchSdkStash(m.user.HomeDir, m.prjId, "ws")
	}

	err := writeSketchSdk(rootdir, meta)
	c.Assert(err, check.IsNil)
	metadir := filepath.Join(rootdir, "meta")
	hooksdir := filepath.Join(rootdir, "hooks")

	return metadir, hooksdir
}

func (m *workshopSketch) mockSketchHappyRefreshPath(c *check.C, refreshname string, refreshMode string) {
	n := 0
	workshop := "ws"
	m.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch n {
		case 1, 3:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects")
			r := fmt.Sprintf(`{"type": "sync", "result": {"id":"%s","path":"%s"}}`, m.prjId, m.prjDir)
			fmt.Fprintln(w, r)
		case 2:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops/%s", m.prjId, workshop))
			w.WriteHeader(200)
			rspcontent := fmt.Sprintf(mockWorkshopWithContent, filepath.Join(m.prjDir, "workshop.yaml"))
			fmt.Fprintln(w, rspcontent)
		case 4:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, map[string]interface{}{"action": "refresh",
				"names": []interface{}{refreshname}, "options": map[string]interface{}{"refresh-mode": refreshMode}})
			w.WriteHeader(202)
			fmt.Fprintln(w, `{"type":"async", "change": "42", "status-code": 202}`)
		case 5:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/changes/42")
			fmt.Fprintln(w, mockReadyChangeJSON)
		default:
			c.Errorf("expected 5 calls, now on %d", n)
		}
	})
}

func (m *workshopSketch) TestSketchSdkMetaOnlySuccess(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}}

	m.mockSketchHappyRefreshPath(c, "ws/sketch", "wait-on-error")

	sketchContent := fmt.Sprintf(sketchTemplate, filepath.Join(m.prjDir, "workshop.yaml"), "ubuntu@22.04")
	restore := MockTextEditor(func(inPath string, inContent []byte) ([]byte, error) {
		return []byte(inContent), nil
	})
	defer restore()

	err := cmd.Run(cmd.Command(), []string{"ws"})
	c.Assert(err, check.IsNil)

	current := sdk.WorkshopSketchSdkCurrent(m.user.HomeDir, m.prjId, "ws")
	c.Assert(filepath.Join(current, "meta", "sdk.yaml"), testutil.FileEquals, sketchContent)
	c.Assert(filepath.Join(current, "hooks"), testutil.FileAbsent)
}

func (m *workshopSketch) TestSketchSdkSuccess(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}}

	m.mockSketchHappyRefreshPath(c, "ws/sketch", "wait-on-error")

	sketchContent := `name: sketch
base: ubuntu@22.04

hooks:
    setup-base: |-
        echo "Hello"
`
	restore := MockTextEditor(func(inPath string, inContent []byte) ([]byte, error) {
		return []byte(sketchContent), nil
	})
	defer restore()

	err := cmd.Run(cmd.Command(), []string{"ws"})
	c.Assert(err, check.IsNil)

	current := sdk.WorkshopSketchSdkCurrent(m.user.HomeDir, m.prjId, "ws")
	c.Assert(filepath.Join(current, "meta", "sdk.yaml"), testutil.FileEquals, sketchContent)
	c.Assert(filepath.Join(current, "hooks", "setup-base"), testutil.FileEquals, "echo \"Hello\"\n")
}

func (m *workshopSketch) TestSketchSdkUpdateHooks(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}}
	m.mockSketchHappyRefreshPath(c, "ws/sketch", "wait-on-error")

	meta, hooks := m.mockMinimalSketchSdk(c, true, []byte(`name: sketch
base: ubuntu@22.04

hooks:
  setup-base: |-
    echo "Hello"
  check-health: |-
    workshopctl set-health okay
`))

	sketchContent := `name: sketch
base: ubuntu@22.04

hooks:
    save-state: |-
        # saves state
    restore-state: |- 
        # restores state
`
	restore := MockTextEditor(func(inPath string, inContent []byte) ([]byte, error) {
		return []byte(sketchContent), nil
	})
	defer restore()

	err := cmd.Run(cmd.Command(), []string{"ws"})
	c.Assert(err, check.IsNil)
	c.Assert(filepath.Join(meta, "sdk.yaml"), testutil.FileEquals, sketchContent)
	c.Assert(filepath.Join(hooks, "setup-base"), testutil.FileAbsent)
	c.Assert(filepath.Join(hooks, "check-health"), testutil.FileAbsent)
	c.Assert(filepath.Join(hooks, "save-state"), testutil.FileEquals, "# saves state\n")
	c.Assert(filepath.Join(hooks, "restore-state"), testutil.FileEquals, "# restores state\n")
}

func (m *workshopSketch) TestSketchSdkEditExistingMeta(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}}

	m.mockSketchHappyRefreshPath(c, "ws/sketch", "wait-on-error")

	metadir := filepath.Join(sdk.WorkshopSketchSdkCurrent(m.user.HomeDir, m.prjId, "ws"), "meta")
	err := os.MkdirAll(metadir, 0755)
	c.Assert(err, check.IsNil)
	err = os.WriteFile(filepath.Join(metadir, "sdk.yaml"), []byte(simpleSketchMeta), 0644)
	c.Assert(err, check.IsNil)

	sketchContent := `name: sketch
base: ubuntu@22.04
plugs:
  gpu:
    interface: gpu
`
	restore := MockTextEditor(func(inPath string, inContent []byte) ([]byte, error) {
		return []byte(sketchContent), nil
	})
	defer restore()

	err = cmd.Run(cmd.Command(), []string{"ws"})
	c.Assert(err, check.IsNil)

	c.Assert(filepath.Join(metadir, "sdk.yaml"), testutil.FileEquals, sketchContent)
}

func (m *workshopSketch) TestSketchSdkStashOK(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}, stash: true}
	restore := sdk.WorkshopSketchSdkStash(m.user.HomeDir, m.prjId, "ws")
	c.Assert(filepath.Join(restore, "meta", "sdk.yaml"), testutil.FileAbsent)

	m.mockSketchHappyRefreshPath(c, "ws", "transactional")
	metadir, _ := m.mockMinimalSketchSdk(c, true, []byte(simpleSketchMeta))

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.IsNil)

	c.Assert(metadir, testutil.FileAbsent)
	c.Assert(filepath.Join(restore, "meta", "sdk.yaml"), testutil.FileEquals, simpleSketchMeta)
}

func (m *workshopSketch) TestSketchSdkOverwritesExistingStash(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}, stash: true}
	stash := sdk.WorkshopSketchSdkStash(m.user.HomeDir, m.prjId, "ws")
	_, stashedhooks := m.mockMinimalSketchSdk(c, false, []byte(`name: sketch
base: ubuntu@18.04
hooks:
    setup-base: |
        touch /home/workshop/stash
    check-health: |
        exit 0
`))

	m.mockSketchHappyRefreshPath(c, "ws", "transactional")
	metadir, _ := m.mockMinimalSketchSdk(c, true, []byte(simpleSketchMeta))

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.IsNil)

	c.Assert(metadir, testutil.FileAbsent)
	c.Assert(filepath.Join(stash, "meta", "sdk.yaml"), testutil.FileEquals, simpleSketchMeta)
	c.Assert(filepath.Join(stashedhooks, "setup-base"), testutil.FileAbsent)
	c.Assert(filepath.Join(stashedhooks, "check-health"), testutil.FileAbsent)
}

func (m *workshopSketch) TestSketchSdkStashRevertOnFail(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}, stash: true}

	n := 0
	workshop := "ws"
	m.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch n {
		case 1, 3:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects")
			r := fmt.Sprintf(`{"type": "sync", "result": {"id":"%s","path":"%s"}}`, m.prjId, m.prjDir)
			fmt.Fprintln(w, r)
		case 2:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops/%s", m.prjId, workshop))
			w.WriteHeader(200)
			rspcontent := fmt.Sprintf(mockWorkshopWithContent, filepath.Join(m.prjDir, "workshop.yaml"))
			fmt.Fprintln(w, rspcontent)
		case 4:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, map[string]interface{}{"action": "refresh",
				"names": []interface{}{workshop}, "options": map[string]interface{}{"refresh-mode": "transactional"}})
			w.WriteHeader(202)
			fmt.Fprintln(w, `{"type":"async", "change": "42", "status-code": 202}`)
		case 5:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/changes/42")
			fmt.Fprintln(w, mockChangeWithError)
		default:
			c.Errorf("expected 5 calls, now on %d", n)
		}
	})

	metadir, _ := m.mockMinimalSketchSdk(c, true, []byte(simpleSketchMeta))

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.NotNil)
	c.Assert(n, check.Equals, 5)

	c.Assert(filepath.Join(metadir, "sdk.yaml"), testutil.FileEquals, simpleSketchMeta)
	restore := sdk.WorkshopSketchSdkStash(m.user.HomeDir, m.prjId, "ws")
	recs, err := os.ReadDir(restore)
	c.Assert(recs, check.HasLen, 0)
	c.Assert(err, check.IsNil)
}

func (m *workshopSketch) TestSketchSdkRestoreOK(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}, restore: true}

	m.mockSketchHappyRefreshPath(c, "ws/sketch", "wait-on-error")
	m.mockMinimalSketchSdk(c, false, []byte(simpleSketchMeta))

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.IsNil)

	current := sdk.WorkshopSketchSdkCurrent(m.user.HomeDir, m.prjId, "ws")
	c.Assert(filepath.Join(current, "meta", "sdk.yaml"), testutil.FileEquals, simpleSketchMeta)
}

func (m *workshopSketch) TestSketchSdkRestoreNoStoredSketch(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}, restore: true}

	m.mockSketchHappyRefreshPath(c, "ws/sketch", "wait-on-error")

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.ErrorMatches, `cannot restore: no stashed 'sketch' SDK found`)
}

func (m *workshopSketch) TestSketchSdkRestoreFailsIfCurrentExists(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}, restore: true}

	m.mockSketchHappyRefreshPath(c, "ws/sketch", "wait-on-error")
	stored := `name: sketch
base: ubuntu@22.04
plugs:
  gpu:
    interface: gpu
`
	// current
	m.mockMinimalSketchSdk(c, true, []byte(simpleSketchMeta))
	// stored
	m.mockMinimalSketchSdk(c, false, []byte(stored))

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.ErrorMatches, `cannot restore: the 'sketch' SDK exists; run 'workshop sketch-sdk --remove' to remove it from the workshop`)
}

func (m *workshopSketch) TestRemoveRemovesSketch(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}}

	m.mockSketchHappyRefreshPath(c, "ws/sketch", "wait-on-error")

	restore := MockTextEditor(func(inPath string, inContent []byte) ([]byte, error) {
		return []byte(inContent), nil
	})
	defer restore()

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.IsNil)

	cmdremove := &CmdRemove{root: &CmdRoot{}}
	n := 0
	m.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch n {
		case 1:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects")
			r := fmt.Sprintf(`{"type": "sync", "result": {"id":"%s","path":"%s"}}`, m.prjId, m.prjDir)
			fmt.Fprintln(w, r)
		case 2:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			w.WriteHeader(202)
			fmt.Fprintln(w, `{"type":"async", "change": "42", "status-code": 202}`)
		case 3:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/changes/42")
			fmt.Fprintln(w, mockReadyChangeJSON)
		default:
			c.Errorf("expected 3 calls, now on %d", n)
		}
	})

	m.ResetStdStreams()
	err = cmdremove.Run(nil, []string{"ws"})
	c.Assert(err, check.IsNil)
	c.Assert(m.stdout.String(), check.Matches, `"ws" removed\n`)

	sketchroot := sdk.WorkshopSketchSdk(m.user.HomeDir, m.prjId, "ws")
	c.Assert(sketchroot, testutil.FileAbsent)
}

func (m *workshopSketch) TestSketchSdkRemoveOK(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}, remove: true}

	m.mockSketchHappyRefreshPath(c, "ws", "transactional")
	m.mockMinimalSketchSdk(c, true, []byte(simpleSketchMeta))

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.IsNil)

	current := sdk.WorkshopSketchSdkCurrent(m.user.HomeDir, m.prjId, "ws")
	c.Assert(current, testutil.FileAbsent)
}

func (m *workshopSketch) TestSketchSdkRemoveCurrentNotExist(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}, remove: true}

	m.mockSketchHappyRefreshPath(c, "ws", "transactional")

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.ErrorMatches, `cannot remove: the 'sketch' SDK doesn't exist`)
}
