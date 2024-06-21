package workshop

import (
	"cmp"
	"fmt"
	"os"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"

	"github.com/canonical/workshop/internal/sdk"
)

type Plug struct {
	Bind string `yaml:"bind"`
}

type SdkRecord struct {
	Name    string          `yaml:"name"`
	Channel string          `yaml:"channel"`
	Plugs   map[string]Plug `yaml:"plugs,omitempty"`
}

type SdkList []SdkRecord

type WorkshopFile struct {
	Name string  `yaml:"name"`
	Base string  `yaml:"base"`
	Sdks SdkList `yaml:"sdks,omitempty"`
}

// *.yaml is the only supported extension for workshop files as the only
// recommended "official" extension: https://yaml.org/faq.html. Also, having a
// single way of naming workshop files avoids unneccesary inconsistencies.
var validWorkshopFilename = regexp.MustCompile(`^\.workshop\.(?P<name>[a-z_][a-z0-9_-]*)\.yaml$`)

func (p SdkList) MarshalYAML() (interface{}, error) {
	type sdkDef struct {
		Channel string          `yaml:"channel"`
		Plugs   map[string]Plug `yaml:"plugs,omitempty"`
	}
	b := map[string]sdkDef{}

	for _, v := range p {
		b[v.Name] = sdkDef{Channel: v.Channel, Plugs: v.Plugs}
	}

	node := &yaml.Node{}
	err := node.Encode(b)
	return node, err
}

func (p *SdkList) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("`sdks` must contain YAML mapping, has %v", value.Kind)
	}
	*p = make([]SdkRecord, len(value.Content)/2)
	seen := map[string]bool{}
	for i := 0; i < len(value.Content); i += 2 {
		var res = &(*p)[i/2]
		var name string
		if err := value.Content[i].Decode(&name); err != nil {
			return err
		} else {
			if _, ok := seen[name]; ok {
				return fmt.Errorf("%q SDK must only be included once", name)
			}
			seen[name] = true
			res.Name = name
		}
		if err := value.Content[i+1].Decode(&res); err != nil {
			return err
		}
	}
	return nil
}

func readWorkshop(pathname string) (*WorkshopFile, error) {
	var err error
	var file WorkshopFile

	buf, err := os.ReadFile(pathname)
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(buf, &file); err != nil {
		return nil, err
	}

	slices.SortFunc(file.Sdks, func(a, b SdkRecord) int {
		return cmp.Compare(a.Name, b.Name)
	})

	if !sdk.ValidName.MatchString(file.Name) {
		return nil, fmt.Errorf("a workshop's name must: (1) start with a letter, (2) include only lower case alpha-numeric or an underscore symbol(s)")
	}

	if !slices.Contains(sdk.ValidBases, file.Base) {
		return nil, fmt.Errorf("unsupported base: %s", file.Base)
	}

	// All bindings must refer to the existing SDKs and meet the name validity
	// checks (at this stage). Later, when SDK metadata will be received, the
	// plugs must be checked again (e.g. ensure all those plugs actually exist).
	for _, s := range file.Sdks {
		for _, p := range s.Plugs {
			comps := strings.Split(p.Bind, ":")
			if len(comps) != 2 {
				return nil, fmt.Errorf("incorrect bind plug reference: %q (use <sdk>:<plug>)", p.Bind)
			}
			if !sdk.ValidName.MatchString(comps[0]) {
				return nil, fmt.Errorf("%q isn't a valid SDK name", comps[0])
			}
			if ixd := slices.IndexFunc(file.Sdks, func(sr SdkRecord) bool { return comps[0] == sr.Name }); ixd == -1 {
				return nil, fmt.Errorf("%q tries to bind to a plug from a non-existing SDK", p.Bind)
			}
		}
	}

	for _, s := range file.Sdks {
		if s.Name == sdk.Agent.String() {
			return nil, fmt.Errorf(`"agent" is a reserved SDK name`)
		}
		if matches := sdk.ValidChannel.FindStringSubmatch(s.Channel); matches != nil {
			continue
		} else {
			return nil, fmt.Errorf("unsupported channel %s for \"%s\"", s.Channel, s.Name)
		}
	}

	return &file, nil
}
