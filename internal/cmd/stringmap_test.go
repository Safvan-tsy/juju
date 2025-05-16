// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package cmd_test

import (
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/internal/cmd"
	"github.com/juju/juju/internal/testhelpers"
)

func TestStringMapSuite(t *stdtesting.T) { tc.Run(t, &StringMapSuite{}) }

type StringMapSuite struct {
	testhelpers.IsolationSuite
}

func (s *StringMapSuite) TestStringMapNilOk(c *tc.C) {
	// note that the map may start out nil
	var values map[string]string
	c.Assert(values, tc.IsNil)
	sm := cmd.StringMap{Mapping: &values}
	err := sm.Set("foo=foovalue")
	c.Assert(err, tc.ErrorIsNil)
	err = sm.Set("bar=barvalue")
	c.Assert(err, tc.ErrorIsNil)

	// now the map is non-nil and filled
	c.Assert(values, tc.DeepEquals, map[string]string{
		"foo": "foovalue",
		"bar": "barvalue",
	})
}

func (s *StringMapSuite) TestStringMapBadVal(c *tc.C) {
	sm := cmd.StringMap{Mapping: &map[string]string{}}
	err := sm.Set("foo")
	c.Assert(err, tc.ErrorMatches, "expected key=value format")
}

func (s *StringMapSuite) TestStringMapDupVal(c *tc.C) {
	sm := cmd.StringMap{Mapping: &map[string]string{}}
	err := sm.Set("bar=somevalue")
	c.Assert(err, tc.ErrorIsNil)
	err = sm.Set("bar=someothervalue")
	c.Assert(err, tc.ErrorMatches, "duplicate key specified")
}

func (s *StringMapSuite) TestStringMapNoValue(c *tc.C) {
	sm := cmd.StringMap{Mapping: &map[string]string{}}
	err := sm.Set("bar=")
	c.Assert(err, tc.ErrorMatches, "key and value must be non-empty")
}

func (s *StringMapSuite) TestStringMapNoKey(c *tc.C) {
	sm := cmd.StringMap{Mapping: &map[string]string{}}
	err := sm.Set("=bar")
	c.Assert(err, tc.ErrorMatches, "key and value must be non-empty")
}
