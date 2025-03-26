package sdk

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/exp/slices"
)

var (
	AllowedBases = []string{"ubuntu@20.04", "ubuntu@22.04", "ubuntu@24.04"}
	SdkName      = regexp.MustCompile(`^(?:[a-z0-9]-?)*[a-z](?:-?[a-z0-9])*$`)
	// Regular expression describing correct plug, slot and interface names.
	validPlugSlotIface = regexp.MustCompile("^[a-z](?:-?[a-z0-9])*$")
)

func Validate(sdk *Info) error {
	if !SdkName.MatchString(sdk.Name) {
		return fmt.Errorf("invalid SDK name %q", sdk.Name)
	}

	if !slices.Contains(AllowedBases, sdk.Base) {
		return fmt.Errorf("invalid SDK base %q; supported bases: %s", sdk.Base, strings.Join(AllowedBases, ", "))
	}

	if sdk.BuildTime != nil && sdk.BuildTime.Location() != time.UTC {
		return fmt.Errorf("invalid SDK build time %q: must be UTC", sdk.BuildTime.Format(time.RFC3339))
	}

	for plugName, plug := range sdk.Plugs {
		if err := ValidatePlugName(plugName); err != nil {
			return err
		}
		if err := ValidateInterfaceName(plug.Interface); err != nil {
			return fmt.Errorf("invalid interface name %q for plug %q", plug.Interface, plugName)
		}
	}
	for slotName, slot := range sdk.Slots {
		if err := ValidateSlotName(slotName); err != nil {
			return err
		}
		if err := ValidateInterfaceName(slot.Interface); err != nil {
			return fmt.Errorf("invalid interface name %q for slot %q", slot.Interface, slotName)
		}
	}
	return nil
}

// ValidatePlug checks if a string can be used as a slot name.
//
// Slot names and plug names within one sdk must have unique names.
func ValidatePlugName(name string) error {
	if !validPlugSlotIface.MatchString(name) {
		return fmt.Errorf("invalid plug name: %q", name)
	}
	return nil
}

// ValidateSlot checks if a string can be used as a slot name.
//
// Slot names and plug names within one sdk must have unique names.
func ValidateSlotName(name string) error {
	if !validPlugSlotIface.MatchString(name) {
		return fmt.Errorf("invalid slot name: %q", name)
	}
	return nil
}

// ValidateInterface checks if a string can be used as an interface name.
func ValidateInterfaceName(name string) error {
	if !validPlugSlotIface.MatchString(name) {
		return fmt.Errorf("invalid interface name: %q", name)
	}
	return nil
}
