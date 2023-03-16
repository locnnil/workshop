package workspace

import (
	"fmt"
	"path/filepath"
	"regexp"

	util "github.com/canonical/workspace/internal"
	"github.com/spf13/afero"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

var SupportedBases = []string{"ubuntu@20.04", "ubuntu@22.04"}
var validName = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
var validChannel = regexp.MustCompile(`^(?P<track>[a-zA-Z0-9\.-]+)/(?P<risk>(stable|candidate|beta|edge))$`)

type workspaceFile struct {
	Name string          `yaml:"name" json:"name"`
	Base string          `yaml:"base" json:"base"`
	Sdks map[string]*Sdk `yaml:"sdks" json:"sdks"`
}

type Sdk struct {
	Name    string `yaml:"name" json:"name"`
	Channel string `yaml:"channel" json:"channel"`
}

func ReadWorkspace(project *Project, name string) (*workspaceFile, error) {
	var err error

	var file = &workspaceFile{}

	buf, err := afero.ReadFile(project.fs, filepath.Join(project.ProjectDirectory(),
		util.ToFileName(name)))

	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(buf, file); err != nil {
		return nil, err
	}

	/* Validate workspace properties */
	if !validName.MatchString(file.Name) {
		return nil, fmt.Errorf("a workspace's name must: (1) start with a letter, (2) include only lower case alpha-numeric or an underscore symbol(s)")
	}

	if !slices.Contains(SupportedBases, file.Base) {
		return nil, fmt.Errorf("unsupported base: %s", file.Base)
	}

	if file.Name != name {
		return nil, fmt.Errorf("%s's file must be named as .workspace.%s.yaml (now: %s)", file.Name, file.Name, util.ToFileName(name))
	}

	for i, k := range file.Sdks {
		k.Name = i
		if matches := validChannel.FindStringSubmatch(k.Channel); matches != nil {
			track := matches[validChannel.SubexpIndex("track")]
			risk := matches[validChannel.SubexpIndex("risk")]
			if risk != "stable" {
				file.Sdks[i].Channel = fmt.Sprintf("%s/stable", track)
				fmt.Printf("Only stable risk levels are supported. Switching to %s for \"%s\"\n", file.Sdks[i].Channel, i)
			}
		} else {
			return nil, fmt.Errorf("unsupported channel %s for \"%s\"", k.Channel, i)
		}
	}

	return file, nil
}
