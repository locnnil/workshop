package sdk

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/metautil"
	"gopkg.in/check.v1"
	"gopkg.in/yaml.v2"
)

type Setup struct {
	Name        string    `json:"name"`
	Channel     string    `json:"channel"`
	Revision    int64     `json:"revision"`
	InstallTime time.Time `json:"install-time,omitzero"`
}

type sdkYaml struct {
	Name  string                 `json:"name"`
	Base  string                 `json:"base"`
	Type  string                 `json:"type"`
	Plugs map[string]interface{} `yaml:"plugs,omitempty"`
	Slots map[string]interface{} `yaml:"slots,omitempty"`
}

type Type string

const (
	Sdk  Type = "sdk"
	Core Type = "core"
)

func (t Type) String() string {
	return string(t)
}

type Info struct {
	ProjectId string
	Workshop  string
	Name      string
	Base      string
	Type      Type
	Channel   string
	Revision  int64

	Plugs map[string]*PlugInfo
	Slots map[string]*SlotInfo
	// Plugs or slots with issues (they are not included in Plugs or Slots)
	BadInterfaces map[string]string
}

func (i *Info) Ref() Ref {
	return Ref{
		ProjectId: i.ProjectId,
		Workshop:  i.Workshop,
		Sdk:       i.Name,
	}
}

type Ref struct {
	ProjectId string
	Workshop  string
	Sdk       string
}

var SanitizePlugsSlots = func(snapInfo *Info) {
	panic("SanitizePlugsSlots function not set")
}

func ReadSdkInfo(yamlData []byte, projectId, workshop string, setup Setup) (*Info, error) {
	var sdkYaml sdkYaml
	err := yaml.Unmarshal(yamlData, &sdkYaml)
	if err != nil {
		return &Info{}, err
	}

	if sdkYaml.Type == "" {
		sdkYaml.Type = Sdk.String()
	}

	sdkInfo := &Info{
		ProjectId:     projectId,
		Workshop:      workshop,
		Name:          sdkYaml.Name,
		Base:          sdkYaml.Base,
		Type:          Type(sdkYaml.Type),
		Plugs:         make(map[string]*PlugInfo),
		Slots:         make(map[string]*SlotInfo),
		BadInterfaces: make(map[string]string),
		Revision:      setup.Revision,
		Channel:       setup.Channel,
	}

	if err := setPlugsFromSdkYaml(&sdkYaml, sdkInfo); err != nil {
		return nil, err
	}

	if err := setSlotsFromSdkYaml(&sdkYaml, sdkInfo); err != nil {
		return nil, err
	}

	SanitizePlugsSlots(sdkInfo)
	return sdkInfo, nil
}

func setPlugsFromSdkYaml(y *sdkYaml, sdk *Info) error {
	for name, data := range y.Plugs {
		iface, label, attrs, err := convertToSlotOrPlugData("plug", name, data)
		if err != nil {
			return err
		}

		sdk.Plugs[name] = &PlugInfo{
			Sdk:       sdk,
			Name:      name,
			Interface: iface,
			Attrs:     attrs,
			Label:     label,
		}
	}

	return nil
}

func setSlotsFromSdkYaml(y *sdkYaml, sdk *Info) error {
	for name, data := range y.Slots {
		iface, label, attrs, err := convertToSlotOrPlugData("slot", name, data)
		if err != nil {
			return err
		}
		sdk.Slots[name] = &SlotInfo{
			Sdk:       sdk,
			Name:      name,
			Interface: iface,
			Attrs:     attrs,
			Label:     label,
		}
	}

	return nil
}

func convertToSlotOrPlugData(plugOrSlot, name string, data interface{}) (iface, label string, attrs map[string]interface{}, err error) {
	iface = name
	switch data.(type) {
	case string:
		return data.(string), "", nil, nil
	case nil:
		return name, "", nil, nil
	case map[interface{}]interface{}:
		for keyData, valueData := range data.(map[interface{}]interface{}) {
			key, ok := keyData.(string)
			if !ok {
				err := fmt.Errorf("%s %q has attribute key that is not a string (found %T)",
					plugOrSlot, name, keyData)
				return "", "", nil, err
			}
			if strings.HasPrefix(key, "$") {
				err := fmt.Errorf("%s %q uses reserved attribute %q", plugOrSlot, name, key)
				return "", "", nil, err
			}
			switch key {
			case "":
				return "", "", nil, fmt.Errorf("%s %q has an empty attribute key", plugOrSlot, name)
			case "interface":
				value, ok := valueData.(string)
				if !ok {
					err := fmt.Errorf("interface name on %s %q is not a string (found %T)",
						plugOrSlot, name, valueData)
					return "", "", nil, err
				}
				iface = value
			case "label":
				value, ok := valueData.(string)
				if !ok {
					err := fmt.Errorf("label of %s %q is not a string (found %T)",
						plugOrSlot, name, valueData)
					return "", "", nil, err
				}
				label = value
			default:
				if attrs == nil {
					attrs = make(map[string]interface{})
				}
				value, err := metautil.NormalizeValue(valueData)
				if err != nil {
					return "", "", nil, fmt.Errorf("attribute %q of %s %q: %v", key, plugOrSlot, name, err)
				}
				attrs[key] = value
			}
		}
		return iface, label, attrs, nil
	default:
		err := fmt.Errorf("%s %q has malformed definition (found %T)", plugOrSlot, name, data)
		return "", "", nil, err
	}
}

// SlotInfo provides information about a slot.
type SlotInfo struct {
	Sdk *Info

	Name      string
	Interface string
	Attrs     map[string]interface{}
	Label     string
}

type AttributeNotFoundError struct{ Err error }

func (e AttributeNotFoundError) Error() string {
	return e.Err.Error()
}

func (e AttributeNotFoundError) Is(target error) bool {
	_, ok := target.(AttributeNotFoundError)
	return ok
}

func lookupAttr(attrs map[string]interface{}, path string) (interface{}, bool) {
	var v interface{}
	comps := strings.FieldsFunc(path, func(r rune) bool { return r == '.' })
	if len(comps) == 0 {
		return nil, false
	}
	v = attrs
	for _, comp := range comps {
		m, ok := v.(map[string]interface{})
		if !ok {
			return nil, false
		}
		v, ok = m[comp]
		if !ok {
			return nil, false
		}
	}

	return v, true
}

func getAttribute(sdkName string, ifaceName string, attrs map[string]interface{}, key string, val interface{}) error {
	v, ok := lookupAttr(attrs, key)
	if !ok {
		return AttributeNotFoundError{fmt.Errorf("sdk %q does not have attribute %q for interface %q", sdkName, key, ifaceName)}
	}

	return metautil.SetValueFromAttribute(sdkName, ifaceName, key, v, val)
}

func (slot *SlotInfo) Attr(key string, val interface{}) error {
	return getAttribute(slot.Sdk.Name, slot.Interface, slot.Attrs, key, val)
}

func (slot *SlotInfo) Lookup(key string) (interface{}, bool) {
	return lookupAttr(slot.Attrs, key)
}

// String returns the representation of the slot as sdk:slot string.
func (slot *SlotInfo) String() string {
	return fmt.Sprintf("%s:%s", slot.Sdk.Name, slot.Name)
}

// PlugInfo provides information about a plug.
type PlugInfo struct {
	Sdk *Info

	Name      string
	Interface string
	Attrs     map[string]interface{}
	Label     string
}

func (plug *PlugInfo) Attr(key string, val interface{}) error {
	return getAttribute(plug.Sdk.Name, plug.Interface, plug.Attrs, key, val)
}

func (plug *PlugInfo) Lookup(key string) (interface{}, bool) {
	return lookupAttr(plug.Attrs, key)
}

func SdkCurrentPath(sdkName string) string {
	return filepath.Join(dirs.WorkshopSdksDir, sdkName, "current")
}

func SdkHooksDir(sdkName string) string {
	return filepath.Join(SdkCurrentPath(sdkName), "sdk", "hooks")
}

func SdkHookPath(sdkName, hookName string) string {
	return filepath.Join(SdkHooksDir(sdkName), hookName)
}

func (s *Setup) Filename() string {
	return filepath.Join(dirs.SdkDir, fmt.Sprintf("%s_%d.sdk", s.Name, s.Revision))
}

func MockSanitizePlugsSlots(f func(sdkInfo *Info)) (restore func()) {
	old := SanitizePlugsSlots
	SanitizePlugsSlots = f
	return func() { SanitizePlugsSlots = old }
}

func MockInfo(c *check.C, yamlText string, projectId, workshop string, setup Setup) *Info {
	restoreSanitize := MockSanitizePlugsSlots(func(sdkInfo *Info) {})
	defer restoreSanitize()
	info, err := ReadSdkInfo([]byte(yamlText), projectId, workshop, setup)
	c.Assert(err, check.IsNil)

	err = Validate(info)
	c.Assert(err, check.IsNil)
	return info
}

func MockInvalidInfo(c *check.C, yamlText string, setup Setup) *Info {
	restoreSanitize := MockSanitizePlugsSlots(func(sdkInfo *Info) {})
	defer restoreSanitize()

	sdkInfo, err := ReadSdkInfo([]byte(yamlText), "invalid", "ws", setup)
	c.Assert(err, check.IsNil)
	err = Validate(sdkInfo)
	c.Assert(err, check.NotNil)
	return sdkInfo
}

// BadInterfacesSummary returns a summary of the problems of bad plugs
// and slots in the sdk.
func BadInterfacesSummary(sdkInfo *Info) string {
	inverted := make(map[string][]string)
	for name, reason := range sdkInfo.BadInterfaces {
		inverted[reason] = append(inverted[reason], name)
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "sdk %q has bad plugs or slots: ", sdkInfo.Name)
	reasons := make([]string, 0, len(inverted))
	for reason := range inverted {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	for _, reason := range reasons {
		names := inverted[reason]
		sort.Strings(names)
		for i, name := range names {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(name)
		}
		fmt.Fprintf(&buf, " (%s); ", reason)
	}
	return strings.TrimSuffix(buf.String(), "; ")
}
