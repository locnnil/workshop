package workspacebackend

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

var SupportedBases = []string{"ubuntu@20.04", "ubuntu@22.04"}
var validName = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
var validChannel = regexp.MustCompile(`^(?P<track>[a-zA-Z0-9\.-]+)/(?P<risk>(stable|candidate|beta|edge))$`)

type Sdk struct {
	Name    string `json:"name"`
	Channel string `json:"channel"`
}

type SdkList []Sdk

type WorkspaceFile struct {
	Name string `yaml:"name" json:"name"`
	Base string `yaml:"base" json:"base"`
	Sdks SdkList
}

func (p *SdkList) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("`sdks` must contain YAML mapping, has %v", value.Kind)
	}
	*p = make([]Sdk, len(value.Content)/2)
	for i := 0; i < len(value.Content); i += 2 {
		var res = &(*p)[i/2]
		if err := value.Content[i].Decode(&res.Name); err != nil {
			return err
		}
		if err := value.Content[i+1].Decode(&res); err != nil {
			return err
		}
	}
	return nil
}

func ReadWorkspace(pathname string) (*WorkspaceFile, error) {
	var err error

	var file = &WorkspaceFile{}

	buf, err := os.ReadFile(pathname)

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

	if WorkspaceFileName(file.Name) != filepath.Base(pathname) {
		return nil, fmt.Errorf("%s's file must be named as .workspace.%s.yaml (now: %s)", file.Name, file.Name, filepath.Base(pathname))
	}

	for _, k := range file.Sdks {
		if matches := validChannel.FindStringSubmatch(k.Channel); matches != nil {
			continue
		} else {
			return nil, fmt.Errorf("unsupported channel %s for \"%s\"", k.Channel, k.Name)
		}
	}

	return file, nil
}
