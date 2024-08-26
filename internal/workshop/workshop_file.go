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

	"github.com/canonical/workshop/internal/sdk"
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
	Bind PlugRef `yaml:"bind,omitempty"`
}

type PlugRef struct {
	Sdk  string
	Name string
}

type SlotRef = PlugRef

func (b *PlugRef) UnmarshalYAML(value *yaml.Node) error {
	var refStr string
	if err := value.Decode(&refStr); err != nil {
		return err
	}

	parts := strings.SplitN(refStr, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid plug or slot reference: %q (use <sdk>:<plug or slot>)", refStr)
	}
	if !workshopName.MatchString(parts[0]) {
		return fmt.Errorf("%q isn't a valid SDK name", parts[0])
	}

	b.Sdk = parts[0]
	b.Name = parts[1]
	return nil
}

func (b PlugRef) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%s:%s", b.Sdk, b.Name), nil
}

type SdkRecord struct {
	Name    string                 `yaml:"name"`
	Channel string                 `yaml:"channel"`
	Plugs   map[string]Plug        `yaml:"plugs,omitempty"`
	Slots   map[string]interface{} `yaml:"slots,omitempty"`
}

type Connection struct {
	PlugRef PlugRef `yaml:"plug"`
	SlotRef SlotRef `yaml:"slot"`
}

type SdkList []SdkRecord

type File struct {
	Name        string       `yaml:"name"`
	Base        string       `yaml:"base"`
	Sdks        SdkList      `yaml:"sdks,omitempty"`
	Connections []Connection `yaml:"connections,omitempty"`
}

func (p SdkList) MarshalYAML() (interface{}, error) {
	type sdkDef struct {
		Channel string                 `yaml:"channel"`
		Plugs   map[string]Plug        `yaml:"plugs,omitempty"`
		Slots   map[string]interface{} `yaml:"slots,omitempty"`
	}
	b := map[string]sdkDef{}

	for _, v := range p {
		b[v.Name] = sdkDef{Channel: v.Channel, Plugs: v.Plugs, Slots: v.Slots}
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

	if err = validateSdks(file.Sdks); err != nil {
		return nil, err
	}

	if err = validateBinding(file.Sdks); err != nil {
		return nil, err
	}

	if err = validateConnections(&file); err != nil {
		return nil, err
	}

	return &file, nil
}

func validateSdks(sdks SdkList) error {
	for _, s := range sdks {
		if slices.Contains(sdkBlacklist, s.Name) {
			return fmt.Errorf("%q is a reserved SDK name", s.Name)
		}

		// an SDK installed from a local source does not have a channel.
		if s.Channel == "" {
			continue
		}
		if matches := channel.FindStringSubmatch(s.Channel); matches == nil {
			return fmt.Errorf("unsupported channel %s for \"%s\"", s.Channel, s.Name)
		}
	}
	return nil
}

func validateBinding(sdks SdkList) error {
	// All bindings must refer to the existing SDKs and meet the name validity
	// checks (at this stage). Later, when SDK metadata will be received, the
	// plugs must be checked again (e.g. ensure all those plugs actually exist).
	var masters map[PlugRef][]PlugRef = make(map[PlugRef][]PlugRef)
	var slaves map[PlugRef]PlugRef = make(map[PlugRef]PlugRef)
	for _, s := range sdks {
		for name, p := range s.Plugs {
			mr := PlugRef{Sdk: p.Bind.Sdk, Name: p.Bind.Name}
			sl := PlugRef{Sdk: s.Name, Name: name}
			masters[mr] = append(masters[mr], sl)
			slaves[sl] = mr

			if !slices.ContainsFunc(sdks, func(sr SdkRecord) bool { return p.Bind.Sdk == sr.Name }) {
				return fmt.Errorf("%q tries to bind to a plug from a non-existing SDK", fmt.Sprintf("%s:%s", p.Bind.Sdk, p.Bind.Name))
			}
			if p.Bind.Sdk == s.Name && p.Bind.Name == name {
				return fmt.Errorf("cannot bind plug %s:%s to itself", s.Name, name)
			}
		}
	}

	// Ensure that there are no "multi-level" binds, e.g. s1 bind to m1 bind to m2.
	slaveKeysOrdered := maps.Keys(slaves)
	slices.SortFunc(slaveKeysOrdered, func(a, b PlugRef) int {
		c := cmp.Compare(a.Sdk, b.Sdk)
		if c == 0 {
			return cmp.Compare(a.Name, b.Name)
		}
		return c
	})
	for _, sl := range slaveKeysOrdered {
		m := slaves[sl]
		if _, ok := masters[sl]; ok {
			return fmt.Errorf("cannot bind %s:%s to %s:%s; plug %s:%s must not be bound", sl.Sdk, sl.Name, m.Sdk, m.Name, sl.Sdk, sl.Name)
		}
	}
	return nil
}

func validateConnections(wfile *File) error {
	for _, conn := range wfile.Connections {
		if !slices.ContainsFunc(wfile.Sdks, func(r SdkRecord) bool { return r.Name == conn.PlugRef.Sdk || conn.PlugRef.Sdk == sdk.Host.String() }) {
			return fmt.Errorf(`invalid plug reference "%s:%s": %q SDK is not found in %q workshop`, conn.PlugRef.Sdk, conn.PlugRef.Name, conn.PlugRef.Sdk, wfile.Name)
		}
		if !slices.ContainsFunc(wfile.Sdks, func(r SdkRecord) bool { return r.Name == conn.SlotRef.Sdk || conn.SlotRef.Sdk == sdk.Host.String() }) {
			return fmt.Errorf(`invalid slot reference "%s:%s": %q SDK is not found in %q workshop`, conn.SlotRef.Sdk, conn.SlotRef.Name, conn.SlotRef.Sdk, wfile.Name)
		}
	}
	return nil
}
