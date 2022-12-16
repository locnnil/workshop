package workspace

import (
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type LaunchTestSuite struct {
	suite.Suite
	Fs afero.Fs
}

func (suite *LaunchTestSuite) SetupTest() {
	suite.Fs = afero.NewMemMapFs()
}

func (suite *LaunchTestSuite) TestWorkspaceLaunchFromFile() {
	_, err := NewWorkspace(suite.Fs, "not_found.yaml")
	assert.Equal(suite.T(), os.IsNotExist(err), true)
}

func (suite *LaunchTestSuite) TestWorkspaceLaunchParser() {
	afero.WriteFile(suite.Fs, ".workspace.translation.yaml",
		[]byte(`name: translation
base: ubuntu@20.04`), 0644)

	_, err := NewWorkspace(suite.Fs, ".workspace.translation.yaml")
	assert.ErrorIs(suite.T(), err, nil)
}

func TestRunLaunchTests(t *testing.T) {
	suite.Run(t, new(LaunchTestSuite))
}
