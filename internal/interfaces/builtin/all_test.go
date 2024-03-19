// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package builtin_test

import (
	"fmt"
	"reflect"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/interfaces/device"
	"github.com/canonical/workshop/internal/interfaces/ifacetest"
	"github.com/canonical/workshop/internal/sdk"
)

type AllSuite struct{}

var _ = Suite(&AllSuite{})

// This section contains a list of *valid* defines that represent methods that
// backends recognize and call. They are in individual interfaces as each
// interface can define a subset that it is interested in providing. Those are,
// essentially, the only valid methods that an interface can have, apart
// from what is defined in the Interface golang interface.
type mountDefiner1 interface {
	MountConnectedPlug(spec *device.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error
}
type mountDefiner2 interface {
	MountConnectedSlot(spec *device.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error
}
type mountDefiner3 interface {
	MountPermanentPlug(spec *device.Specification, plug *sdk.PlugInfo) error
}
type mountDefiner4 interface {
	MountPermanentSlot(spec *device.Specification, slot *sdk.SlotInfo) error
}

// allGoodDefiners contains all valid specification definers for all known backends.
var allGoodDefiners = []reflect.Type{
	// mount
	reflect.TypeOf((*mountDefiner1)(nil)).Elem(),
	reflect.TypeOf((*mountDefiner2)(nil)).Elem(),
	reflect.TypeOf((*mountDefiner3)(nil)).Elem(),
	reflect.TypeOf((*mountDefiner4)(nil)).Elem(),
}

// Check that each interface defines at least one definer method we recognize.
func (s *AllSuite) TestEachInterfaceImplementsSomeBackendMethods(c *C) {
	for _, iface := range builtin.Interfaces() {
		bogus := true
		for _, definer := range allGoodDefiners {
			if reflect.TypeOf(iface).Implements(definer) {
				bogus = false
				break
			}
		}
		c.Assert(bogus, Equals, false,
			Commentf("interface %q does not implement any specification methods", iface.Name()))
	}
}

func (s *AllSuite) TestRegisterIface(c *C) {
	restore := builtin.MockInterfaces(nil)
	defer restore()

	// Registering an interface works correctly.
	iface := &ifacetest.TestInterface{InterfaceName: "foo"}
	builtin.RegisterIface(iface)
	c.Assert(builtin.Interface("foo"), DeepEquals, iface)

	// Duplicates are detected.
	c.Assert(func() { builtin.RegisterIface(iface) }, PanicMatches, `cannot register duplicate interface "foo"`)
}

const testConsumerInvalidSlotNameYaml = `
name: consumer
slots:
 ttyS5:
  interface: iface
`

const testConsumerInvalidPlugNameYaml = `
name: consumer
plugs:
 ttyS3:
  interface: iface
`

func (s *AllSuite) TestSanitizeErrorsOnInvalidSlotNames(c *C) {
	restore := builtin.MockInterfaces(map[string]interfaces.Interface{
		"iface": &ifacetest.TestInterface{InterfaceName: "iface"},
	})
	defer restore()

	sdkInfo := sdk.MockInvalidInfo(c, testConsumerInvalidSlotNameYaml)
	sdk.SanitizePlugsSlots(sdkInfo)
	c.Assert(sdkInfo.BadInterfaces, HasLen, 1)
	c.Check(sdk.BadInterfacesSummary(sdkInfo), Matches, `sdk "consumer" has bad plugs or slots: ttyS5 \(invalid slot name: "ttyS5"\)`)
}

func (s *AllSuite) TestSanitizeErrorsOnInvalidPlugNames(c *C) {
	restore := builtin.MockInterfaces(map[string]interfaces.Interface{
		"iface": &ifacetest.TestInterface{InterfaceName: "iface"},
	})
	defer restore()

	sdkInfo := sdk.MockInvalidInfo(c, testConsumerInvalidPlugNameYaml)
	sdk.SanitizePlugsSlots(sdkInfo)
	c.Assert(sdkInfo.BadInterfaces, HasLen, 1)
	c.Check(sdk.BadInterfacesSummary(sdkInfo), Matches, `sdk "consumer" has bad plugs or slots: ttyS3 \(invalid plug name: "ttyS3"\)`)
}

func (s *AllSuite) TestUnexpectedSpecSignatures(c *C) {
	type funcSig struct {
		name string
		in   []string
		out  []string
	}
	var sigs []funcSig

	// All the valid signatures from all the specification definers from all the backends.
	for _, backend := range []string{string(interfaces.SecurityLxdDevice)} {
		backendLower := strings.ToLower(backend)
		sigs = append(sigs, []funcSig{{
			name: fmt.Sprintf("%sPermanentPlug", backend),
			in: []string{
				fmt.Sprintf("*%s.Specification", backendLower),
				"*sdk.PlugInfo",
			},
			out: []string{"error"},
		}, {
			name: fmt.Sprintf("%sPermanentSlot", backend),
			in: []string{
				fmt.Sprintf("*%s.Specification", backendLower),
				"*sdk.SlotInfo",
			},
			out: []string{"error"},
		}, {
			name: fmt.Sprintf("%sConnectedPlug", backend),
			in: []string{
				fmt.Sprintf("*%s.Specification", backendLower),
				"*interfaces.ConnectedPlug",
				"*interfaces.ConnectedSlot",
			},
			out: []string{"error"},
		}, {
			name: fmt.Sprintf("%sConnectedSlot", backend),
			in: []string{
				fmt.Sprintf("*%s.Specification", backendLower),
				"*interfaces.ConnectedPlug",
				"*interfaces.ConnectedSlot",
			},
			out: []string{"error"},
		}}...)
	}
	for _, iface := range builtin.Interfaces() {
		ifaceVal := reflect.ValueOf(iface)
		ifaceType := ifaceVal.Type()
		for _, sig := range sigs {
			meth, ok := ifaceType.MethodByName(sig.name)
			if !ok {
				// all specification methods are optional.
				continue
			}
			methType := meth.Type
			// Check that the signature matches our expectation. The -1 and +1 below is for the receiver type.
			c.Assert(methType.NumIn()-1, Equals, len(sig.in), Commentf("expected %s's %s method to take %d arguments", ifaceType, meth.Name, len(sig.in)))
			for i, expected := range sig.in {
				c.Assert(methType.In(i+1).String(), Equals, expected, Commentf("expected %s's %s method %dth argument type to be different", ifaceType, meth.Name, i))
			}
			c.Assert(methType.NumOut(), Equals, len(sig.out), Commentf("expected %s's %s method to return %d values", ifaceType, meth.Name, len(sig.out)))
			for i, expected := range sig.out {
				c.Assert(methType.Out(i).String(), Equals, expected, Commentf("expected %s's %s method %dth return value type to be different", ifaceType, meth.Name, i))
			}
		}
	}
}
