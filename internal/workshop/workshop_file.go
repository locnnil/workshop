package workshop

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"

	"github.com/canonical/workshop/internal/sdk"
)

var (
	SupportedBases = []string{"ubuntu@20.04", "ubuntu@22.04", "ubuntu@24.04"}

	workshopName = regexp.MustCompile(`^[a-z](?:-?[a-z0-9])*$`)
	channel      = regexp.MustCompile(`^(?:[a-z0-9](?:[.-]?[a-z0-9])*/(?:stable|candidate|beta|edge)|)$`)
	scriptName   = workshopName

	Directory = ".workshop"
	Filenames = []string{"workshop.yaml", ".workshop.yaml"}
)

func filename(name string) string {
	return fmt.Sprintf("%s.yaml", name)
}

func Filepath(project, name string) string {
	return filepath.Join(project, Directory, filename(name))
}

func ProjectSdkPath(project, name string) string {
	return filepath.Join(project, Directory, name)
}

type PlugOrBind struct {
	Bind *PlugRef
	Plug interface{}
}

func (p *PlugOrBind) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		p.Bind = nil
		return value.Decode(&p.Plug)
	}

	var plug struct {
		Bind       *PlugRef               `yaml:"bind"`
		Attributes map[string]interface{} `yaml:",inline"`
	}
	if err := value.Decode(&plug); err != nil {
		return err
	}

	if plug.Bind == nil {
		*p = PlugOrBind{Plug: plug.Attributes}
		return nil
	}

	if len(plug.Attributes) > 0 {
		return fmt.Errorf("plug is bound to %q and must not define other attributes", plug.Bind.String())
	}
	*p = PlugOrBind{Bind: plug.Bind}
	return nil
}

func (p PlugOrBind) MarshalYAML() (interface{}, error) {
	if p.Bind == nil {
		return p.Plug, nil
	}

	if p.Plug != nil {
		return nil, fmt.Errorf("plug is bound to %q and must not define other attributes", p.Bind.String())
	}

	bind, err := p.Bind.MarshalYAML()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"bind": bind}, nil
}

type PlugRef struct {
	Sdk  string
	Name string
}

func (p PlugRef) String() string {
	return fmt.Sprintf("%s:%s", p.Sdk, p.Name)
}

type SlotRef = PlugRef

func (b *PlugRef) UnmarshalYAML(value *yaml.Node) error {
	var refStr string
	if err := value.Decode(&refStr); err != nil {
		return err
	}

	parts := strings.Split(refStr, ":")
	if len(parts) != 2 {
		return fmt.Errorf("%q is not a valid plug or slot reference (use <sdk>:<plug or slot>)", refStr)
	}
	if len(parts[0]) == 0 {
		parts[0] = sdk.System.String()
	}
	if err := sdk.ValidateName(parts[0]); err != nil {
		return fmt.Errorf("%q is not a valid plug or slot reference: %w", refStr, err)
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
	Channel string                 `yaml:"channel,omitempty"`
	Source  sdk.Source             `yaml:"source,omitempty"`
	Plugs   map[string]PlugOrBind  `yaml:"plugs,omitempty"`
	Slots   map[string]interface{} `yaml:"slots,omitempty"`
}

func (s SdkRecord) MarshalYAML() (interface{}, error) {
	if s.Source == sdk.ProjectSource {
		s.Name = fmt.Sprintf("project-%s", s.Name)
	}
	s.Source = 0

	type record SdkRecord
	return (*record)(&s), nil
}

func (s *SdkRecord) UnmarshalYAML(value *yaml.Node) error {
	type record SdkRecord
	err := value.Decode((*record)(s))

	name, found := strings.CutPrefix(s.Name, "project-")
	if found {
		s.Name = name
		s.Source = sdk.ProjectSource
	} else if sdk.IsSystem(s.Name) {
		s.Source = sdk.SystemSource
	} else if sdk.IsSketch(s.Name) {
		s.Source = sdk.SketchSource
	} else {
		s.Source = sdk.StoreSource
	}

	return err
}

type Connection struct {
	PlugRef PlugRef `yaml:"plug"`
	SlotRef SlotRef `yaml:"slot"`
}

type Script string

type File struct {
	Name        string            `yaml:"name"`
	Base        string            `yaml:"base"`
	Sdks        []SdkRecord       `yaml:"sdks,omitempty"`
	Connections []Connection      `yaml:"connections,omitempty"`
	Scripts     map[string]Script `yaml:"scripts,omitempty"`
}

func (p Script) String() string {
	// Trim newlines, then append a newline for multi-line scripts.
	script := strings.Trim(string(p), "\n")
	if strings.ContainsRune(script, '\n') {
		script += "\n"
	}
	return script
}

func (p Script) MarshalYAML() (interface{}, error) {
	node := &yaml.Node{}
	err := node.Encode(p.String())
	return node, err
}

func (p *Script) UnmarshalYAML(value *yaml.Node) error {
	var script string
	if err := value.Decode(&script); err != nil {
		return err
	}

	// Scripts should have trailing newlines,
	// but YAML one-liners don't.
	if !strings.HasSuffix(script, "\n") {
		script += "\n"
	}

	// Adjust line numbers to improve error messages.
	// Only accurate for literals (|) and one-liners.
	if (value.Kind & yaml.ScalarNode) != 0 {
		line := value.Line
		if (value.Style & (yaml.LiteralStyle | yaml.FoldedStyle)) != 0 {
			line += 1
		}
		if line > 1 {
			script = strings.Repeat("\n", line-1) + script
		}
	}

	*p = Script(script)
	return nil
}

func readWorkshop(path string) (*File, error) {
	var err error
	var file File

	buf, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("workshop definition %q not found", path)
		}
		return nil, err
	}
	if err = yaml.Unmarshal(buf, &file); err != nil {
		te, ok := err.(*yaml.TypeError)
		if ok {
			errs := strings.Join(te.Errors, "\n")
			return nil, fmt.Errorf("workshop definition YAML:\n%s", errs)
		}
		return nil, err
	}

	if !workshopName.MatchString(file.Name) {
		return nil, fmt.Errorf("a workshop's name must: (1) start with a letter, (2) only include digits, lowercase letters, and hyphens joining them")
	}

	if !slices.Contains(SupportedBases, file.Base) {
		return nil, fmt.Errorf("base %q not supported", file.Base)
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

	if err = validateScripts(&file); err != nil {
		return nil, err
	}

	return &file, nil
}

func validateSdks(sdks []SdkRecord) error {
	seen := map[string]bool{}
	for _, s := range sdks {
		if err := sdk.ValidateName(s.Name); err != nil {
			return err
		}

		if _, ok := seen[s.Name]; ok {
			return fmt.Errorf("%q SDK must only be included once", s.Name)
		}
		seen[s.Name] = true

		if !channel.MatchString(s.Channel) {
			return fmt.Errorf("unsupported channel %q for %q SDK", s.Channel, s.Name)
		}
	}
	return nil
}

func validateBinding(sdks []SdkRecord) error {
	// All bindings must refer to the existing SDKs and meet the name validity
	// checks (at this stage). Later, when SDK metadata will be received, the
	// plugs must be checked again (e.g. ensure all those plugs actually exist).
	masters := make(map[PlugRef][]PlugRef)
	slaves := make(map[PlugRef]PlugRef)
	for _, s := range sdks {
		for name, p := range s.Plugs {
			if p.Bind == nil {
				continue
			}
			mr := *p.Bind
			sl := PlugRef{Sdk: s.Name, Name: name}
			masters[mr] = append(masters[mr], sl)
			slaves[sl] = mr

			if sdk.IsSystem(sl.Sdk) {
				return fmt.Errorf("cannot bind system SDK plug %q", sl.String())
			}
			if sdk.IsSystem(mr.Sdk) {
				return fmt.Errorf("cannot bind to system SDK plug %q", mr.String())
			}
			if !IsImplicitSdk(p.Bind.Sdk) && !slices.ContainsFunc(sdks, func(sr SdkRecord) bool { return p.Bind.Sdk == sr.Name }) {
				return fmt.Errorf("cannot bind plug %q: SDK %q not found", p.Bind.String(), p.Bind.Sdk)
			}
			if mr == sl {
				return fmt.Errorf(`cannot bind plug %q to itself`, p.Bind.String())
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
			return fmt.Errorf(`cannot bind %q to %q: plug %q is already bound`, sl.String(), m.String(), sl.String())
		}
	}
	return nil
}

func isBound(plug PlugRef, wf *File) bool {
	return slices.ContainsFunc(wf.Sdks, func(s SdkRecord) bool {
		if s.Name != plug.Sdk {
			return false
		}

		for name, p := range s.Plugs {
			if name == plug.Name && p.Bind != nil {
				return true
			}
		}
		return false
	})
}

func validateConnections(wfile *File) error {
	for _, conn := range wfile.Connections {
		if isBound(conn.PlugRef, wfile) {
			return fmt.Errorf(`cannot connect plug %q to slot %q: plug is bound`,
				conn.PlugRef.String(), conn.SlotRef.String())
		}

		if !slices.ContainsFunc(wfile.Sdks, func(r SdkRecord) bool { return r.Name == conn.PlugRef.Sdk || IsImplicitSdk(conn.PlugRef.Sdk) }) {
			return fmt.Errorf(`cannot connect plug %q to slot %q: workshop %q has no SDK named %q`,
				conn.PlugRef.String(), conn.SlotRef.String(), wfile.Name, conn.PlugRef.Sdk)
		}
		if !slices.ContainsFunc(wfile.Sdks, func(r SdkRecord) bool { return r.Name == conn.SlotRef.Sdk || IsImplicitSdk(conn.SlotRef.Sdk) }) {
			return fmt.Errorf(`cannot connect plug %q to slot %q: workshop %q has no SDK named %q`,
				conn.PlugRef.String(), conn.SlotRef.String(), wfile.Name, conn.SlotRef.Sdk)
		}
	}
	return nil
}

func validateScripts(wfile *File) error {
	for name := range wfile.Scripts {
		if !scriptName.MatchString(name) {
			return fmt.Errorf("script name %q must: (1) start with a letter, (2) only include digits, lowercase letters, and hyphens joining them", name)
		}
	}
	return nil
}

// IsImplicitSdk checks whether the given SDK is installed
// regardless of whether it appears in the workshop file.
func IsImplicitSdk(name string) bool {
	return sdk.IsSystem(name) || sdk.IsSketch(name)
}

// IsProjectSdk checks whether the given SDK is defined
// in the project directory.
func IsProjectSdk(name string) bool {
	return strings.HasPrefix(name, "project-")
}
