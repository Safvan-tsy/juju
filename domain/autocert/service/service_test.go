// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	stdtesting "testing"

	"github.com/juju/tc"
	gomock "go.uber.org/mock/gomock"
	"golang.org/x/crypto/acme/autocert"

	coreerrors "github.com/juju/juju/core/errors"
	"github.com/juju/juju/internal/errors"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/testhelpers"
)

type serviceSuite struct {
	testhelpers.IsolationSuite

	state *MockState
}

func TestServiceSuite(t *stdtesting.T) {
	tc.Run(t, &serviceSuite{})
}

func (s *serviceSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.state = NewMockState(ctrl)

	return ctrl
}

func (s *serviceSuite) TestCheckCacheMiss(c *tc.C) {
	defer s.setupMocks(c).Finish()

	certName := "test-cert-name"
	s.state.EXPECT().Get(gomock.Any(), certName).Return(nil, errors.Errorf("autocert %s: %w", certName, coreerrors.NotFound))

	svc := NewService(s.state, loggertesting.WrapCheckLog(c))

	certbytes, err := svc.Get(c.Context(), certName)
	c.Assert(certbytes, tc.IsNil)
	c.Assert(err, tc.ErrorIs, autocert.ErrCacheMiss)
}

func (s *serviceSuite) TestCheckAnyError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	certName := "test-cert-name"
	s.state.EXPECT().Get(gomock.Any(), certName).Return(nil, errors.New("state error"))

	svc := NewService(s.state, loggertesting.WrapCheckLog(c))

	certbytes, err := svc.Get(c.Context(), certName)
	c.Assert(certbytes, tc.IsNil)
	c.Assert(err, tc.ErrorMatches, "state error")
}
