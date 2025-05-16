// Copyright 2022 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charms

import (
	stdtesting "testing"

	"github.com/juju/errors"
	"github.com/juju/tc"

	"github.com/juju/juju/rpc/params"
)

type charmOriginSuite struct{}

func TestCharmOriginSuite(t *stdtesting.T) { tc.Run(t, &charmOriginSuite{}) }
func (s *charmOriginSuite) TestValidateCharmOriginSuccessCharmHub(c *tc.C) {
	err := ValidateCharmOrigin(&params.CharmOrigin{
		Hash:   "myHash",
		ID:     "myID",
		Source: "charm-hub",
	})
	c.Assert(err, tc.Not(tc.ErrorIs), errors.BadRequest)
}

func (s *charmOriginSuite) TestValidateCharmOriginSuccessLocal(c *tc.C) {
	err := ValidateCharmOrigin(&params.CharmOrigin{Source: "local"})
	c.Assert(err, tc.Not(tc.ErrorIs), errors.BadRequest)
}

func (s *charmOriginSuite) TestValidateCharmOriginNil(c *tc.C) {
	err := ValidateCharmOrigin(nil)
	c.Assert(err, tc.ErrorIs, errors.BadRequest)
}

func (s *charmOriginSuite) TestValidateCharmOriginNilSource(c *tc.C) {
	err := ValidateCharmOrigin(&params.CharmOrigin{Source: ""})
	c.Assert(err, tc.ErrorIs, errors.BadRequest)
}

func (s *charmOriginSuite) TestValidateCharmOriginBadSource(c *tc.C) {
	err := ValidateCharmOrigin(&params.CharmOrigin{Source: "charm-store"})
	c.Assert(err, tc.ErrorIs, errors.BadRequest)
}

func (s *charmOriginSuite) TestValidateCharmOriginCharmHubIDNoHash(c *tc.C) {
	err := ValidateCharmOrigin(&params.CharmOrigin{
		ID:     "myID",
		Source: "charm-hub",
	})
	c.Assert(err, tc.ErrorIs, errors.BadRequest)
}

func (s *charmOriginSuite) TestValidateCharmOriginCharmHubHashNoID(c *tc.C) {
	err := ValidateCharmOrigin(&params.CharmOrigin{
		Hash:   "myHash",
		Source: "charm-hub",
	})
	c.Assert(err, tc.ErrorIs, errors.BadRequest)
}
