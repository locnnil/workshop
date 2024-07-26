package workshop

import (
	"cmp"
	"fmt"
	"os"
	"regexp"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

var (
	SupportedBases = []string{"ubuntu@20.04", "ubuntu@22.04", "ubuntu@24.04"}
	workshopName   = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
	channel        = regexp.MustCompile(`^(?P<track>[a-zA-Z0-9\.-]+)/(?P<risk>(stable|candidate|beta|edge))$`)

	// *.yaml is the only supported extension for workshop files as the only
	// recommended "official" extension: https://yaml.org/faq.html. Also, having a
	// single way of naming workshop files avoids unneccesary inconsistencies.
	filename     = regexp.MustCompile(`^\.workshop\.(?P<name>[a-z_][a-z0-9_-]*)\.yaml$`)
	sdkBlacklist = []string{"agent"}
)

type Plug struct {
	Bind Bind `yaml:"bind"`
}

type Bind struct {
	Sdk  string
	Plug string
}

func (b *Bind) UnmarshalYAML(value *yaml.Node) error {
	var bindStr string
	if err := value.Decode(&bindStr); err != nil {
		return err
	}

	parts := strings.SplitN(bindStr, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("incorrect bind plug reference: %q (use <sdk>:<plug>)", bindStr)
	}
	if !workshopName.MatchString(parts[0]) {
		return fmt.Errorf("%q isn't a valid SDK name", parts[0])
	}

	b.Sdk = parts[0]
	b.Plug = parts[1]
	return nil
}

func (b Bind) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%s:%s", b.Sdk, b.Plug), nil
}

type SdkRecord struct {
	Name    string          `yaml:"name"`
	Channel string          `yaml:"channel"`
	Plugs   map[string]Plug `yaml:"plugs,omitempty"`
}

type SdkList []SdkRecord

type File struct {
	Name string  `yaml:"name"`
	Base string  `yaml:"base"`
	Sdks SdkList `yaml:"sdks,omitempty"`
}

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

func readWorkshop(pathname string) (*File, error) {
	var err error
	var file File

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

	if !workshopName.MatchString(file.Name) {
		return nil, fmt.Errorf("a workshop's name must: (1) start with a letter, (2) include only lower case alpha-numeric or an underscore symbol(s)")
	}

	if !slices.Contains(SupportedBases, file.Base) {
		return nil, fmt.Errorf("unsupported base: %s", file.Base)
	}

	// All bindings must refer to the existing SDKs and meet the name validity
	// checks (at this stage). Later, when SDK metadata will be received, the
	// plugs must be checked again (e.g. ensure all those plugs actually exist).
	type plug struct {
		sdk  string
		name string
	}
	var masters map[plug][]plug = make(map[plug][]plug)
	var slaves map[plug]plug = make(map[plug]plug)
	for _, s := range file.Sdks {
		for name, p := range s.Plugs {
			mr := plug{sdk: p.Bind.Sdk, name: p.Bind.Plug}
			sl := plug{sdk: s.Name, name: name}
			masters[mr] = append(masters[mr], sl)
			slaves[sl] = mr

			if ixd := slices.IndexFunc(file.Sdks, func(sr SdkRecord) bool { return p.Bind.Sdk == sr.Name }); ixd == -1 {
				return nil, fmt.Errorf("%q tries to bind to a plug from a non-existing SDK", fmt.Sprintf("%s:%s", p.Bind.Sdk, p.Bind.Plug))
			}
			if p.Bind.Sdk == s.Name && p.Bind.Plug == name {
				return nil, fmt.Errorf("cannot bind plug %s:%s to itself", s.Name, name)
			}
		}
	}

	// Ensure that there are no "multi-level" binds, e.g. s1 bind to m1 bind to m2.
	slaveKeysOrdered := maps.Keys(slaves)
	slices.SortFunc(slaveKeysOrdered, func(a, b plug) int {
		c := cmp.Compare(a.sdk, b.sdk)
		if c == 0 {
			return cmp.Compare(a.name, b.name)
		}
		return c
	})
	for _, sl := range slaveKeysOrdered {
		m := slaves[sl]
		if _, ok := masters[sl]; ok {
			return nil, fmt.Errorf("cannot bind %s:%s to %s:%s; plug %s:%s must not be bound", sl.sdk, sl.name, m.sdk, m.name, sl.sdk, sl.name)
		}
	}

	for _, s := range file.Sdks {
		if idx := slices.Index(sdkBlacklist, s.Name); idx != -1 {
			return nil, fmt.Errorf(`"agent" is a reserved SDK name`)
		}
		if matches := channel.FindStringSubmatch(s.Channel); matches != nil {
			continue
		} else {
			return nil, fmt.Errorf("unsupported channel %s for \"%s\"", s.Channel, s.Name)
		}
	}

	return &file, nil
}
