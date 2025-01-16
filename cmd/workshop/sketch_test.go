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

var mockWorkshopsListWithSketch = `{"type":"sync","status-code":200,"status":"OK","result":{"workshops":[{"name":"ws","base":"ubuntu@22.04","project-id":"42424242","status":"Ready","sdks":[{"name":"sketch","channel":"","revision":"x1","install-time":"2017-03-22T09:01:00.0Z"}]},{"name":"nosketch","base":"ubuntu@22.04","project-id":"42424242","status":"Ready"},{"name":"both","base":"ubuntu@22.04","project-id":"42424242","status":"Ready","sdks":[{"name":"sketch","channel":"","revision":"x3","install-time":"2017-03-22T09:01:00.0Z"}]},{"name":"none","base":"ubuntu@22.04","project-id":"42424242","status":"Ready"}]},"warning-timestamp":"2017-03-22T10:01:00.0Z","warning-count":1}`

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

func (m *workshopSketch) mockMinimalSketchSdk(c *check.C, ws string, current bool, meta []byte) (metapath string, hookspath string) {
	var rootdir string
	if current {
		rootdir = sdk.WorkshopSketchSdkCurrent(m.user.HomeDir, m.prjId, ws)
	} else {
		rootdir = sdk.WorkshopSketchSdkStash(m.user.HomeDir, m.prjId, ws)
	}

	err := writeSketchSdk(rootdir, meta)
	c.Assert(err, check.IsNil)
	metadir := filepath.Join(rootdir, "meta")
	hooksdir := filepath.Join(rootdir, "hooks")

	return metadir, hooksdir
}

func (m *workshopSketch) mockSketchHappyRefreshPath(c *check.C, refreshname string, mode string) {
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
			fmt.Fprintln(w, mockWorkshopWithSdks)
		case 4:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, map[string]interface{}{"action": "refresh",
				"names": []interface{}{refreshname}, "options": map[string]interface{}{"mode": mode}})
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

func (m *workshopSketch) mockSketchesHappyPath(c *check.C, resp string) {
	n := 0
	m.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch n {
		case 1:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects")
			res := fmt.Sprintf(`{"type": "sync", "result": {"id":"%s","path":"%s"}}`, m.prjId, m.prjDir)
			fmt.Fprintln(w, res)
		case 2:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects/"+m.prjId+"/workshops")
			w.WriteHeader(200)
			fmt.Fprintln(w, resp)
		default:
			c.Errorf("expected 2 calls, now on %d", n)
		}
	})
}

func (m *workshopSketch) TestSketchSdkMetaOnlySuccess(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}}

	m.mockSketchHappyRefreshPath(c, "ws/sketch", "wait-on-error")

	sketchContent := fmt.Sprintf(sketchTemplate, "/home/project/.workshop/ws.yaml", "ubuntu@22.04")
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

	meta, hooks := m.mockMinimalSketchSdk(c, "ws", true, []byte(`name: sketch
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

func (m *workshopSketch) TestSketchSdkFixRefreshError(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}}

	// Runs sketch-sdk with a failing setup-base hook,
	// then immediately re-runs it to fix the hook.
	// The second run should automatically abort the first refresh.
	// The API calls break down as follows:
	// 1-2: get workshop info
	// 3-5: refresh --wait-on-error (fails due to setup-base)
	// 6-7: get workshop info
	// 8-9. refresh --wait-on-error (fails due to earlier refresh)
	// 10-12. refresh --abort
	// 13-15. refresh --wait-on-error
	n := 0
	change := 42
	workshop := "ws"
	m.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch n {
		case 1, 3, 6, 8, 10, 13:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects")
			r := fmt.Sprintf(`{"type": "sync", "result": {"id":"%s","path":"%s"}}`, m.prjId, m.prjDir)
			fmt.Fprintln(w, r)
		case 2, 7:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops/%s", m.prjId, workshop))
			w.WriteHeader(200)
			fmt.Fprintln(w, mockWorkshopWithSdks)
		case 4, 9, 11, 14:
			mode := "wait-on-error"
			name := fmt.Sprintf("%s/sketch", workshop)
			if n == 11 {
				mode = "abort"
				name = workshop
			}

			change += 1
			status := 202
			response := fmt.Sprintf(`{"type":"async", "change": "%d", "status-code": 202}`, change)
			if n == 9 {
				status = 400
				response = `{"type":"error", "result": {"message":"already waiting on error", "kind":"waiting-on-error"}, "status-code": 400}`
			}

			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, map[string]interface{}{"action": "refresh",
				"names": []interface{}{name}, "options": map[string]interface{}{"mode": mode}})
			w.WriteHeader(status)
			fmt.Fprintln(w, response)
		case 5, 12, 15:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/changes/%d", change))
			if n == 5 {
				fmt.Fprintln(w, mockWaitChangeJSON)
			} else if n == 12 {
				fmt.Fprintln(w, mockAbortedChangeJSON)
			} else {
				fmt.Fprintln(w, mockReadyChangeJSON)
			}
		default:
			c.Errorf("expected 15 calls, now on %d", n)
		}
	})

	attempts := 0
	sketchSetup := `name: sketch
base: ubuntu@22.04

hooks:
    setup-base: |
        %s
`
	restore := MockTextEditor(func(inPath string, inContent []byte) ([]byte, error) {
		attempts += 1
		switch attempts {
		case 1:
			return []byte(fmt.Sprintf(sketchSetup, "false")), nil
		case 2:
			return []byte(fmt.Sprintf(sketchSetup, "true")), nil
		default:
			return nil, fmt.Errorf("expected 2 attempts, now on %d", attempts)
		}
	})
	defer restore()

	err := cmd.Run(cmd.Command(), []string{"ws"})
	c.Assert(err, check.ErrorMatches, `cannot refresh; fix the errors reported,\nthen run "workshop refresh --continue ws".\nTo abort and revert, run "workshop refresh --abort ws"`)

	err = cmd.Run(cmd.Command(), []string{"ws"})
	c.Assert(err, check.IsNil)

	c.Assert(n, check.Equals, 15)
	c.Assert(attempts, check.Equals, 2)
}

func (m *workshopSketch) TestSketchSdkStashOK(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}, stash: true}
	restore := sdk.WorkshopSketchSdkStash(m.user.HomeDir, m.prjId, "ws")
	c.Assert(filepath.Join(restore, "meta", "sdk.yaml"), testutil.FileAbsent)

	m.mockSketchHappyRefreshPath(c, "ws", "transactional")
	metadir, _ := m.mockMinimalSketchSdk(c, "ws", true, []byte(simpleSketchMeta))

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.IsNil)

	c.Assert(metadir, testutil.FileAbsent)
	c.Assert(filepath.Join(restore, "meta", "sdk.yaml"), testutil.FileEquals, simpleSketchMeta)
}

func (m *workshopSketch) TestSketchSdkOverwritesExistingStash(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}, stash: true}
	stash := sdk.WorkshopSketchSdkStash(m.user.HomeDir, m.prjId, "ws")
	_, stashedhooks := m.mockMinimalSketchSdk(c, "ws", false, []byte(`name: sketch
base: ubuntu@18.04
hooks:
    setup-base: |
        touch /home/workshop/stash
    check-health: |
        exit 0
`))

	m.mockSketchHappyRefreshPath(c, "ws", "transactional")
	metadir, _ := m.mockMinimalSketchSdk(c, "ws", true, []byte(simpleSketchMeta))

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
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			w.WriteHeader(200)
			fmt.Fprintln(w, mockSingleWorkshop)
		case 4:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, map[string]interface{}{"action": "refresh",
				"names": []interface{}{workshop}, "options": map[string]interface{}{"mode": "transactional"}})
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

	metadir, _ := m.mockMinimalSketchSdk(c, "ws", true, []byte(simpleSketchMeta))

	err := cmd.Run(nil, nil)
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
	m.mockMinimalSketchSdk(c, "ws", false, []byte(simpleSketchMeta))

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
	m.mockMinimalSketchSdk(c, "ws", true, []byte(simpleSketchMeta))
	// stored
	m.mockMinimalSketchSdk(c, "ws", false, []byte(stored))

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
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			fmt.Fprintln(w, mockSingleWorkshop)
		case 3:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			w.WriteHeader(202)
			fmt.Fprintln(w, `{"type":"async", "change": "42", "status-code": 202}`)
		case 4:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/changes/42")
			fmt.Fprintln(w, mockReadyChangeJSON)
		default:
			c.Errorf("expected 4 calls, now on %d", n)
		}
	})

	m.ResetStdStreams()
	err = cmdremove.Run(nil, nil)
	c.Assert(err, check.IsNil)
	c.Assert(m.stdout.String(), check.Matches, `"ws" removed\n`)
	c.Check(n, check.Equals, 4)

	sketchroot := sdk.WorkshopSketchSdk(m.user.HomeDir, m.prjId, "ws")
	c.Assert(sketchroot, testutil.FileAbsent)
}

func (m *workshopSketch) TestSketchSdkRemoveOK(c *check.C) {
	cmd := &CmdSketch{root: &CmdRoot{}, remove: true}

	m.mockSketchHappyRefreshPath(c, "ws", "transactional")
	m.mockMinimalSketchSdk(c, "ws", true, []byte(simpleSketchMeta))

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

func (m *workshopSketch) TestSketchesOK(c *check.C) {
	cmd := &CmdSketches{root: &CmdRoot{}}

	m.mockSketchesHappyPath(c, mockWorkshopsListWithSketch)
	m.mockMinimalSketchSdk(c, "ws", true, []byte(simpleSketchMeta))
	m.mockMinimalSketchSdk(c, "nosketch", false, []byte(simpleSketchMeta))
	m.mockMinimalSketchSdk(c, "both", false, []byte(simpleSketchMeta))

	err := cmd.Run(nil, nil)
	c.Assert(err, check.IsNil)

	c.Assert(m.stdout.String(), check.Matches, fmt.Sprintf(`Project +Workshop  Rev  Notes
%s  ws        x1   current
%s  nosketch  -    stashed
%s  both      x3   current,stashed
`, m.prjDir, m.prjDir, m.prjDir))
}

func (m *workshopSketch) TestSketchesEmpty(c *check.C) {
	cmd := &CmdSketches{root: &CmdRoot{}}

	m.mockSketchesHappyPath(c, mockWorkshopList)

	err := cmd.Run(nil, nil)
	c.Assert(err, check.IsNil)

	c.Assert(m.stdout.String(), check.Matches, "")
}
