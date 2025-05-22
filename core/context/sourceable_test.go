// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package context

import (
	"context"
	stdtesting "testing"

	"github.com/juju/tc"
	"gopkg.in/tomb.v2"

	"github.com/juju/juju/internal/errors"
	"github.com/juju/juju/internal/testhelpers"
)

type contextSuite struct {
	testhelpers.IsolationSuite
}

func TestContextSuite(t *stdtesting.T) {
	tc.Run(t, &contextSuite{})
}

func (s *contextSuite) TestSourceableErrorIsNilIfErrorIsNotContextError(c *tc.C) {
	var tomb tomb.Tomb
	tomb.Kill(errors.New("tomb error"))

	// We only want to propagate the sourceable error if the error is a
	// context error. Otherwise you can always check the error with the
	// source directly.

	ctx := WithSourceableError(c.Context(), &tomb)
	err := ctx.Err()
	c.Assert(err, tc.ErrorIsNil)
}

func (s *contextSuite) TestSourceableErrorIsIgnoredIfNotInErrorState(c *tc.C) {
	var tomb tomb.Tomb

	ctx, cancel := context.WithCancel(c.Context())
	cancel()

	ctx = WithSourceableError(ctx, &tomb)
	err := ctx.Err()
	c.Assert(err, tc.ErrorIs, context.Canceled)
}

func (s *contextSuite) TestSourceableErrorIsTombError(c *tc.C) {
	var tomb tomb.Tomb
	tomb.Kill(errors.New("boom"))

	ctx, cancel := context.WithCancel(c.Context())
	cancel()

	ctx = WithSourceableError(ctx, &tomb)
	err := ctx.Err()
	c.Assert(err, tc.ErrorMatches, `boom`)
}

func (s *contextSuite) TestSourceableErrorIsTiedToTheTomb(c *tc.C) {
	var tomb tomb.Tomb

	ctx := tomb.Context(c.Context())

	tomb.Kill(errors.New("boom"))

	ctx = WithSourceableError(ctx, &tomb)
	err := ctx.Err()
	c.Assert(err, tc.ErrorMatches, `boom`)
}
