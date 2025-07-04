// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"

	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/testhelpers"
)

//go:generate go run go.uber.org/mock/mockgen -typed -package service -destination package_mock_test.go github.com/juju/juju/domain/removal/service State,Provider
//go:generate go run go.uber.org/mock/mockgen -typed -package service -destination leadership_mock_test.go github.com/juju/juju/core/leadership Revoker
//go:generate go run go.uber.org/mock/mockgen -typed -package service -destination clock_mock_test.go github.com/juju/clock Clock

type baseSuite struct {
	testhelpers.IsolationSuite

	state    *MockState
	clock    *MockClock
	revoker  *MockRevoker
	provider *MockProvider
}

func (s *baseSuite) newService(c *tc.C) *Service {
	return &Service{
		st:                s.state,
		leadershipRevoker: s.revoker,
		provider: func(ctx context.Context) (Provider, error) {
			return s.provider, nil
		},
		clock:  s.clock,
		logger: loggertesting.WrapCheckLog(c),
	}
}

func (s *baseSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.state = NewMockState(ctrl)
	s.clock = NewMockClock(ctrl)
	s.revoker = NewMockRevoker(ctrl)
	s.provider = NewMockProvider(ctrl)

	c.Cleanup(func() {
		s.state = nil
		s.clock = nil
		s.revoker = nil
		s.provider = nil
	})

	return ctrl
}
