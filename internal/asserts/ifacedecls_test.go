package asserts_test

import (
	"fmt"
	"strings"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/asserts"
)

var (
	_ = check.Suite(&plugSlotRulesSuite{})
)

var (
	sideArityAny = asserts.SideArityConstraint{N: -1}
	sideArityOne = asserts.SideArityConstraint{N: 1}
)

type plugSlotRulesSuite struct{}

func (s *plugSlotRulesSuite) TestCompileSlotRuleInstallationConstraintsSlotNames(c *check.C) {
	m, err := asserts.ParseHeaders([]byte(`iface:
  allow-installation: true`))
	c.Assert(err, check.IsNil)

	rule, err := asserts.CompileSlotRule("iface", m["iface"].(map[string]interface{}))
	c.Assert(err, check.IsNil)

	c.Check(rule.AllowInstallation[0].SlotNames, check.IsNil)

	tests := []struct {
		rule        string
		matching    []string
		notMatching []string
	}{
		{`iface:
  allow-installation:
    slot-names:
      - foo`, []string{"foo"}, []string{"bar"}},
		{`iface:
  allow-installation:
    slot-names:
      - foo
      - bar`, []string{"foo", "bar"}, []string{"baz"}},
		{`iface:
  allow-installation:
    slot-names:
      - foo[0-9]
      - bar`, []string{"foo0", "foo1", "bar"}, []string{"baz", "fooo", "foo12"}},
	}
	for _, t := range tests {
		m, err = asserts.ParseHeaders([]byte(t.rule))
		c.Assert(err, check.IsNil)

		rule, err = asserts.CompileSlotRule("iface", m["iface"].(map[string]interface{}))
		c.Assert(err, check.IsNil)

		for _, matching := range t.matching {
			c.Check(rule.AllowInstallation[0].SlotNames.Check("slot name", matching, nil), check.IsNil)
		}
		for _, notMatching := range t.notMatching {
			c.Check(rule.AllowInstallation[0].SlotNames.Check("slot name", notMatching, nil), check.NotNil)
		}
	}
}

func (s *plugSlotRulesSuite) TestCompileSlotRuleConnectionConstraintsPlugNamesSlotNames(c *check.C) {
	m, err := asserts.ParseHeaders([]byte(`iface:
  allow-connection: true`))
	c.Assert(err, check.IsNil)

	rule, err := asserts.CompileSlotRule("iface", m["iface"].(map[string]interface{}))
	c.Assert(err, check.IsNil)

	c.Check(rule.AllowConnection[0].PlugNames, check.IsNil)
	c.Check(rule.AllowConnection[0].SlotNames, check.IsNil)

	tests := []struct {
		rule        string
		matching    []string
		notMatching []string
	}{
		{`iface:
  allow-connection:
    plug-names:
      - Pfoo
    slot-names:
      - Sfoo`, []string{"foo"}, []string{"bar"}},
		{`iface:
  allow-connection:
    plug-names:
      - Pfoo
      - Pbar
    slot-names:
      - Sfoo
      - Sbar`, []string{"foo", "bar"}, []string{"baz"}},
		{`iface:
  allow-connection:
    plug-names:
      - Pfoo[0-9]
      - Pbar
    slot-names:
      - Sfoo[0-9]
      - Sbar`, []string{"foo0", "foo1", "bar"}, []string{"baz", "fooo", "foo12"}},
	}
	for _, t := range tests {
		m, err = asserts.ParseHeaders([]byte(t.rule))
		c.Assert(err, check.IsNil)

		rule, err = asserts.CompileSlotRule("iface", m["iface"].(map[string]interface{}))
		c.Assert(err, check.IsNil)

		for _, matching := range t.matching {
			c.Check(rule.AllowConnection[0].PlugNames.Check("plug name", "P"+matching, nil), check.IsNil)

			c.Check(rule.AllowConnection[0].SlotNames.Check("slot name", "S"+matching, nil), check.IsNil)
		}

		for _, notMatching := range t.notMatching {
			c.Check(rule.AllowConnection[0].SlotNames.Check("plug name", "P"+notMatching, nil), check.NotNil)

			c.Check(rule.AllowConnection[0].SlotNames.Check("slot name", "S"+notMatching, nil), check.NotNil)
		}
	}
}

func (s *plugSlotRulesSuite) TestCompileSlotRuleInstallationConstraintsIDConstraints(c *check.C) {
	rule, err := asserts.CompileSlotRule("iface", map[string]interface{}{
		"allow-installation": map[string]interface{}{
			"slot-sdk-type": []interface{}{"host", "regular"},
		},
	})
	c.Assert(err, check.IsNil)

	c.Assert(rule.AllowInstallation, check.HasLen, 1)
	cstrs := rule.AllowInstallation[0]
	c.Check(cstrs.SlotTypes, check.DeepEquals, []string{"host", "regular"})
}

func checkBoolSlotConnConstraints(c *check.C, subrule string, cstrs []*asserts.SlotConnectionConstraints, always bool) {
	c.Assert(cstrs, check.HasLen, 1)
	cstrs1 := cstrs[0]
	if strings.HasPrefix(subrule, "deny-") {
		undef := asserts.SideArityConstraint{}
		c.Check(cstrs1.SlotsPerPlug, check.Equals, undef)
		c.Check(cstrs1.PlugsPerSlot, check.Equals, undef)
	} else {
		c.Check(cstrs1.PlugsPerSlot, check.Equals, sideArityAny)
		if strings.HasSuffix(subrule, "-auto-connection") {
			c.Check(cstrs1.SlotsPerPlug, check.Equals, sideArityOne)
		} else {
			c.Check(cstrs1.SlotsPerPlug, check.Equals, sideArityAny)
		}
	}
	c.Check(cstrs1.PlugSdkTypes, check.HasLen, 0)
}

func (s *plugSlotRulesSuite) TestCompileSlotRuleDefaults(c *check.C) {
	rule, err := asserts.CompileSlotRule("iface", map[string]interface{}{
		"deny-auto-connection": "true",
	})
	c.Assert(err, check.IsNil)

	// everything follows the defaults...

	// install subrules
	c.Assert(rule.AllowInstallation, check.HasLen, 1)

	// connection subrules
	checkBoolSlotConnConstraints(c, "allow-connection", rule.AllowConnection, true)
	checkBoolSlotConnConstraints(c, "deny-connection", rule.DenyConnection, false)
	// auto-connection subrules
	checkBoolSlotConnConstraints(c, "allow-auto-connection", rule.AllowAutoConnection, true)
	// ... but deny-auto-connection is on
	checkBoolSlotConnConstraints(c, "deny-auto-connection", rule.DenyAutoConnection, true)
}

func (s *plugSlotRulesSuite) TestCompileSlotRuleConnectionConstraintsSideArityConstraints(c *check.C) {
	m, err := asserts.ParseHeaders([]byte(`iface:
  allow-auto-connection: true`))
	c.Assert(err, check.IsNil)

	rule, err := asserts.CompileSlotRule("iface", m["iface"].(map[string]interface{}))
	c.Assert(err, check.IsNil)

	// defaults
	c.Check(rule.AllowAutoConnection[0].SlotsPerPlug, check.Equals, asserts.SideArityConstraint{N: 1})
	c.Check(rule.AllowAutoConnection[0].PlugsPerSlot.Any(), check.Equals, true)

	c.Check(rule.AllowConnection[0].SlotsPerPlug.Any(), check.Equals, true)
	c.Check(rule.AllowConnection[0].PlugsPerSlot.Any(), check.Equals, true)

	// test that the arity constraints get normalized away to any
	// under allow-connection
	// see https://forum.snapcraft.io/t/plug-slot-declaration-rules-greedy-plugs/12438
	allowConnTests := []string{
		`iface:
  allow-connection:
    slots-per-plug: 1
    plugs-per-slot: 2`,
		`iface:
  allow-connection:
    slots-per-plug: *
    plugs-per-slot: 1`,
		`iface:
  allow-connection:
    slots-per-plug: 2
    plugs-per-slot: *`,
	}

	for _, t := range allowConnTests {
		m, err = asserts.ParseHeaders([]byte(t))
		c.Assert(err, check.IsNil)

		rule, err = asserts.CompileSlotRule("iface", m["iface"].(map[string]interface{}))
		c.Assert(err, check.IsNil)

		c.Check(rule.AllowConnection[0].SlotsPerPlug.Any(), check.Equals, true)
		c.Check(rule.AllowConnection[0].PlugsPerSlot.Any(), check.Equals, true)
	}

	// test that under allow-auto-connection:
	// slots-per-plug can be * (any) or otherwise gets normalized to 1
	// plugs-per-slot gets normalized to any (*)
	// see https://forum.snapcraft.io/t/plug-slot-declaration-rules-greedy-plugs/12438
	allowAutoConnTests := []struct {
		rule         string
		slotsPerPlug asserts.SideArityConstraint
	}{
		{`iface:
  allow-auto-connection:
    slots-per-plug: 1
    plugs-per-slot: 2`, sideArityOne},
		{`iface:
  allow-auto-connection:
    slots-per-plug: *
    plugs-per-slot: 1`, sideArityAny},
		{`iface:
  allow-auto-connection:
    slots-per-plug: 2
    plugs-per-slot: *`, sideArityOne},
	}

	for _, t := range allowAutoConnTests {
		m, err = asserts.ParseHeaders([]byte(t.rule))
		c.Assert(err, check.IsNil)

		rule, err = asserts.CompileSlotRule("iface", m["iface"].(map[string]interface{}))
		c.Assert(err, check.IsNil)

		c.Assert(rule.AllowAutoConnection[0].SlotsPerPlug, check.Equals, t.slotsPerPlug)
		c.Check(rule.AllowAutoConnection[0].PlugsPerSlot.Any(), check.Equals, true)
	}
}

func (s *plugSlotRulesSuite) TestCompileSlotRuleErrors(c *check.C) {
	tests := []struct {
		stanza string
		err    string
	}{
		{`iface: foo`, `slot rule for interface "iface" must be a map or one of the shortcuts 'true' or 'false'`},
		{`iface:
  - allow`, `slot rule for interface "iface" must be a map or one of the shortcuts 'true' or 'false'`},
		{`iface:
  allow-installation: foo`, `allow-installation in slot rule for interface "iface" must be a map or one of the shortcuts 'true' or 'false'`},
		{`iface:
  deny-installation: foo`, `deny-installation in slot rule for interface "iface" must be a map or one of the shortcuts 'true' or 'false'`},
		{`iface:
  allow-connection: foo`, `allow-connection in slot rule for interface "iface" must be a map or one of the shortcuts 'true' or 'false'`},
		{`iface:
  allow-connection:
    - foo`, `alternative 1 of allow-connection in slot rule for interface "iface" must be a map`},
		{`iface:
  allow-connection:
    - true`, `alternative 1 of allow-connection in slot rule for interface "iface" must be a map`},
		{`iface:
  allow-connection:
    plug-sdk-type:
      - foo`, `plug-sdk-type in allow-connection in slot rule for interface "iface" contains an invalid element: "foo"`},
		{`iface:
  allow-connection:
    plug-sdk-type:
      - xapp`, `plug-sdk-type in allow-connection in slot rule for interface "iface" contains an invalid element: "xapp"`},
		{`iface:
  allow-connection:
    plug-sdk-ids:
      - foo`, `allow-connection in slot rule for interface "iface" must specify at least one of plug-names, slot-names, plug-attributes, slot-attributes, plug-sdk-type, slots-per-plug, plugs-per-slot`},
		{`iface:
  deny-connection:
    plug-sdk-ids:
        - foo`, `deny-connection in slot rule for interface "iface" must specify at least one of plug-names, slot-names, plug-attributes, slot-attributes, plug-sdk-type, slots-per-plug, plugs-per-slot`},
		{`iface:
  allow-connect: true`, `slot rule for interface "iface" must specify at least one of allow-installation, deny-installation, allow-connection, deny-connection, allow-auto-connection, deny-auto-connection`},
		{`iface:
  allow-installation:
    slots-per-plug: 1`, `allow-installation in slot rule for interface "iface" cannot specify a slots-per-plug constraint, they apply only to allow-\*connection`},
		{`iface:
  deny-auto-connection:
    slots-per-plug: 1`, `deny-auto-connection in slot rule for interface "iface" cannot specify a slots-per-plug constraint, they apply only to allow-\*connection`},
		{`iface:
  deny-auto-connection:
    plug-sdk-ids:
      - foo`, `deny-auto-connection in slot rule for interface "iface" must specify at least one of plug-names, slot-names, plug-attributes, slot-attributes, plug-sdk-type, slots-per-plug, plugs-per-slot`},
		{`iface:
  allow-auto-connection:
    plugs-per-slot: any`, `plugs-per-slot in allow-auto-connection in slot rule for interface "iface" must be an integer >=1 or \*`},
		{`iface:
  allow-auto-connection:
    slots-per-plug: 00`, `slots-per-plug in allow-auto-connection in slot rule for interface "iface" has invalid prefix zeros: 00`},
		{`iface:
  allow-auto-connection:
    slots-per-plug: 99999999999999999999`, `slots-per-plug in allow-auto-connection in slot rule for interface "iface" is out of range: 99999999999999999999`},
		{`iface:
  allow-auto-connection:
    slots-per-plug: 0`, `slots-per-plug in allow-auto-connection in slot rule for interface "iface" must be an integer >=1 or \*`},
		{`iface:
  allow-auto-connection:
    slots-per-plug:
      what: 1`, `slots-per-plug in allow-auto-connection in slot rule for interface "iface" must be an integer >=1 or \*`},
		{`iface:
  allow-auto-connection:
    slot-names: true`, `slot-names constraints must be a list of regexps and special \$ values`},
		{`iface:
  allow-auto-connection:
    plug-sdk-ids:
      - foo`, `allow-auto-connection in slot rule for interface "iface" must specify at least one of plug-names, slot-names, plug-attributes, slot-attributes, plug-sdk-type, slots-per-plug, plugs-per-slot`},
	}

	for i, t := range tests {
		m, err := asserts.ParseHeaders([]byte(t.stanza))
		c.Assert(err, check.IsNil, check.Commentf(t.stanza))
		_, err = asserts.CompileSlotRule("iface", m["iface"])
		c.Assert(err, check.ErrorMatches, t.err, check.Commentf("%d: %s", i, t.stanza))
	}
}

type nameConstraintsSuite struct{}

func (s *nameConstraintsSuite) TestCompileErrors(c *check.C) {
	_, err := asserts.CompileNameConstraints("slot-names", "true")
	c.Check(err, check.ErrorMatches, `slot-names constraints must be a list of regexps and special \$ values`)

	_, err = asserts.CompileNameConstraints("slot-names", []interface{}{map[string]interface{}{"foo": "bar"}})
	c.Check(err, check.ErrorMatches, `slot-names constraint entry must be a regexp or special \$ value`)

	_, err = asserts.CompileNameConstraints("plug-names", []interface{}{"["})
	c.Check(err, check.ErrorMatches, `cannot compile plug-names constraint entry "\[":.*`)

	_, err = asserts.CompileNameConstraints("plug-names", []interface{}{"$"})
	c.Check(err, check.ErrorMatches, `plug-names constraint entry special value "\$" is invalid`)

	_, err = asserts.CompileNameConstraints("slot-names", []interface{}{"$12"})
	c.Check(err, check.ErrorMatches, `slot-names constraint entry special value "\$12" is invalid`)

	_, err = asserts.CompileNameConstraints("plug-names", []interface{}{"a b"})
	c.Check(err, check.ErrorMatches, `plug-names constraint entry regexp contains unexpected spaces`)
}

func (s *nameConstraintsSuite) TestCheck(c *check.C) {
	nc, err := asserts.CompileNameConstraints("slot-names", []interface{}{"foo[0-9]", "bar"})
	c.Assert(err, check.IsNil)

	for _, matching := range []string{"foo0", "foo1", "bar"} {
		c.Check(nc.Check("slot name", matching, nil), check.IsNil)
	}

	for _, notMatching := range []string{"baz", "fooo", "foo12"} {
		c.Check(nc.Check("slot name", notMatching, nil), check.ErrorMatches, fmt.Sprintf(`slot name %q does not match constraints`, notMatching))
	}

}

func (s *nameConstraintsSuite) TestCheckSpecial(c *check.C) {
	nc, err := asserts.CompileNameConstraints("slot-names", []interface{}{"$INTERFACE"})
	c.Assert(err, check.IsNil)

	c.Check(nc.Check("slot name", "foo", nil), check.ErrorMatches, `slot name "foo" does not match constraints`)
	c.Check(nc.Check("slot name", "foo", map[string]string{"$INTERFACE": "foo"}), check.IsNil)
	c.Check(nc.Check("slot name", "bar", map[string]string{"$INTERFACE": "foo"}), check.ErrorMatches, `slot name "bar" does not match constraints`)
}
