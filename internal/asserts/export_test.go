// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package asserts

import "io"

// Headers helpers to test
var (
	ParseHeaders                = parseHeaders
	CompilePlugRule             = compilePlugRule
	CompileSlotRule             = compileSlotRule
	CompileNameConstraints      = compileNameConstraints
	CompileAttributeConstraints = compileAttributeConstraints
)

func init() {
	maxSupportedFormat[TestOnlyType.Name] = 1

	typeRegistry[TestOnlyType.Name] = TestOnlyType
	typeRegistry[TestOnly2Type.Name] = TestOnly2Type
	typeRegistry[TestOnlyNoAuthorityType.Name] = TestOnlyNoAuthorityType
}

// define test assertion types to use in the tests

type TestOnly struct {
	assertionBase
}

func assembleTestOnly(assert assertionBase) (Assertion, error) {
	// for testing error cases
	if _, err := checkIntWithDefault(assert.headers, "count", 0); err != nil {
		return nil, err
	}
	return &TestOnly{assert}, nil
}

var TestOnlyType = &AssertionType{"test-only", []string{"primary-key"}, nil, assembleTestOnly, 0}

type TestOnly2 struct {
	assertionBase
}

func assembleTestOnly2(assert assertionBase) (Assertion, error) {
	return &TestOnly2{assert}, nil
}

var TestOnly2Type = &AssertionType{"test-only-2", []string{"pk1", "pk2"}, nil, assembleTestOnly2, 0}

type TestOnlyNoAuthority struct {
	assertionBase
}

func assembleTestOnlyNoAuthority(assert assertionBase) (Assertion, error) {
	if _, err := checkNotEmptyString(assert.headers, "hdr"); err != nil {
		return nil, err
	}
	return &TestOnlyNoAuthority{assert}, nil
}

var TestOnlyNoAuthorityType = &AssertionType{"test-only-no-authority", nil, nil, assembleTestOnlyNoAuthority, noAuthority}

// NewDecoderStressed makes a Decoder with a stressed setup with the given buffer and maximum sizes.
func NewDecoderStressed(r io.Reader, bufSize, maxHeadersSize, maxBodySize, maxSigSize int) *Decoder {
	return (&Decoder{
		rd:                 r,
		initialBufSize:     bufSize,
		maxHeadersSize:     maxHeadersSize,
		maxSigSize:         maxSigSize,
		defaultMaxBodySize: maxBodySize,
	}).initBuffer()
}

func CompileAttrMatcher(constraints any, allowedOperations []string) (func(attrs map[string]any, helper AttrMatchContext) error, error) {
	// XXX adjust
	cc := compileContext{
		opts: &compileAttrMatcherOptions{
			allowedOperations: allowedOperations,
		},
	}
	matcher, err := compileAttrMatcher(cc, constraints)
	if err != nil {
		return nil, err
	}
	domatch := func(attrs map[string]any, helper AttrMatchContext) error {
		return matcher.match("", attrs, &attrMatchingContext{
			attrWord: "field",
			helper:   helper,
		})
	}
	return domatch, nil
}

type featureExposer interface {
	feature(flabel string) bool
}

func RuleFeature(rule featureExposer, flabel string) bool {
	return rule.feature(flabel)
}
