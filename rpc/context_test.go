// Copyright 2023, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package rpc_test

import (
	"testing"

	"github.com/juju/tc"

	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/rpc"
)

type contextSuite struct {
	testhelpers.IsolationSuite
}

func TestContextSuite(t *testing.T) {
	tc.Run(t, &contextSuite{})
}

func (s *contextSuite) TestWithTracing(c *tc.C) {
	ctx := rpc.WithTracing(c.Context(), "trace", "span", 1)
	traceID, spanID, flags := rpc.TracingFromContext(ctx)
	c.Assert(traceID, tc.Equals, "trace")
	c.Assert(spanID, tc.Equals, "span")
	c.Assert(flags, tc.Equals, 1)
}
