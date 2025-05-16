// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model_test

import (
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/cmd/juju/model"
	"github.com/juju/juju/internal/cmd/cmdtesting"
	"github.com/juju/juju/internal/testing"
)

type ModelConstraintsCommandsSuite struct {
	testing.FakeJujuXDGDataHomeSuite
}

func TestModelConstraintsCommandsSuite(t *stdtesting.T) {
	tc.Run(t, &ModelConstraintsCommandsSuite{})
}
func (s *ModelConstraintsCommandsSuite) TestSetInit(c *tc.C) {
	for _, test := range []struct {
		args []string
		err  string
	}{
		{
			args: []string{"-s", "mysql"},
			err:  "option provided but not defined: -s",
		}, {
			args: []string{"="},
			err:  `malformed constraint "="`,
		}, {
			args: []string{"cpu-power=250"},
		},
	} {
		err := cmdtesting.InitCommand(model.NewModelSetConstraintsCommandForTest(), test.args)
		if test.err == "" {
			c.Check(err, tc.ErrorIsNil)
		} else {
			c.Check(err, tc.ErrorMatches, test.err)
		}
	}
}

func (s *ModelConstraintsCommandsSuite) TestGetInit(c *tc.C) {
	for _, test := range []struct {
		args []string
		err  string
	}{
		{
			args: []string{"-s", "mysql"},
			err:  "option provided but not defined: -s",
		}, {
			args: []string{"mysql"},
			err:  `unrecognized args: \["mysql"\]`,
		}, {
			args: []string{},
		},
	} {
		err := cmdtesting.InitCommand(model.NewModelGetConstraintsCommandForTest(), test.args)
		if test.err == "" {
			c.Check(err, tc.ErrorIsNil)
		} else {
			c.Check(err, tc.ErrorMatches, test.err)
		}
	}
}
