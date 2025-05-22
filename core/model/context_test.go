// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	stdtesting "testing"

	"github.com/juju/tc"
)

type contextSuite struct{}

func TestContextSuite(t *stdtesting.T) {
	tc.Run(t, &contextSuite{})
}

func (s *contextSuite) TestContextModelUUIDIsPassed(c *tc.C) {
	ctx := WithContextModelUUID(c.Context(), UUID("model-uuid"))
	modelUUID, ok := ModelUUIDFromContext(ctx)
	c.Assert(ok, tc.Equals, true)
	c.Check(modelUUID, tc.Equals, UUID("model-uuid"))
}
