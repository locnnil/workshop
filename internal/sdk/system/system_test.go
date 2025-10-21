package system_test

import (
	"crypto/sha3"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/sdk/system"
	"github.com/canonical/workshop/internal/testutil"
)

type systemSdk struct {
	oldRoot  string
	oldCache string
}

var _ = check.Suite(&systemSdk{})

func Test(t *testing.T) { check.TestingT(t) }

func (f *systemSdk) SetUpSuite(c *check.C) {
	f.oldRoot = dirs.BaseDir
	f.oldCache = dirs.CacheDir
	dirs.SetRootDir(c.MkDir())
	dirs.SetCacheDir(c.MkDir())
	c.Assert(dirs.CreateDirs(), check.IsNil)
}

func (f *systemSdk) TearDownSuite(c *check.C) {
	dirs.SetCacheDir(f.oldRoot)
	dirs.SetRootDir(f.oldRoot)
}

func (s *systemSdk) TestRetrieveSystemSdkSuccess(c *check.C) {
	done, total := 0, 0
	report := &progress.Reporter{Name: "1", Report: func(label string, d, t int) {
		done = d
		total = t
	}}
	setup := sdk.Setup{Name: sdk.System.String(), Source: sdk.SystemSource, Revision: system.SystemSdkRevision}
	result, err := system.RetrieveSystemSdk(setup, report)
	c.Assert(err, check.IsNil)
	c.Check(result.Setup, check.Equals, setup)
	c.Check(result.Sha3_384, check.Equals, system.SystemSdkDigest)
	c.Check(result.SdkYAML, check.Not(check.Equals), "")
	c.Check(done, testutil.IntGreaterThan, 0)
	c.Check(total, testutil.IntGreaterThan, 0)
	c.Check(done, check.Equals, total)

	r, err := os.Open(setup.Filepath())
	c.Assert(err, check.IsNil)
	defer r.Close()

	hash := sha3.New384()
	_, err = r.WriteTo(hash)
	c.Assert(err, check.IsNil)

	digest := hex.EncodeToString(hash.Sum(nil))
	c.Check(digest, check.Equals, system.SystemSdkDigest, check.Commentf("system SDK revision needs updating"))
	c.Check(system.SystemSdkRevision, check.Equals, sdk.R(1))
}

func (s *systemSdk) TestRetrieveSystemSdkWrongRevision(c *check.C) {
	setup := sdk.Setup{Name: sdk.System.String(), Source: sdk.SystemSource, Revision: sdk.R(system.SystemSdkRevision.N - 1)}
	_, err := system.RetrieveSystemSdk(setup, nil)
	c.Check(err, check.ErrorMatches, fmt.Sprintf(`system SDK \(%s\) not available`, setup.Revision))

	setup.Revision.N += 2
	_, err = system.RetrieveSystemSdk(setup, nil)
	c.Check(err, check.ErrorMatches, fmt.Sprintf(`system SDK \(%s\) not available`, setup.Revision))
}
