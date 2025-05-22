// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package secretspruner_test

import (
	"testing"

	"github.com/juju/errors"
	"github.com/juju/tc"
	"github.com/juju/worker/v4"
	dt "github.com/juju/worker/v4/dependency/testing"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/api/base"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/internal/worker/secretspruner"
	"github.com/juju/juju/internal/worker/secretspruner/mocks"
)

type manifoldSuite struct {
	testhelpers.IsolationSuite
	config secretspruner.ManifoldConfig
}

func TestManifoldSuite(t *testing.T) {
	tc.Run(t, &manifoldSuite{})
}

func (s *manifoldSuite) SetUpTest(c *tc.C) {
	s.IsolationSuite.SetUpTest(c)
	s.config = s.validConfig(c)
}

func (s *manifoldSuite) validConfig(c *tc.C) secretspruner.ManifoldConfig {
	return secretspruner.ManifoldConfig{
		APICallerName: "api-caller",
		Logger:        loggertesting.WrapCheckLog(c),
		NewWorker: func(config secretspruner.Config) (worker.Worker, error) {
			return nil, nil
		},
		NewUserSecretsFacade: func(base.APICaller) secretspruner.SecretsFacade { return nil },
	}
}

func (s *manifoldSuite) TestValid(c *tc.C) {
	c.Check(s.config.Validate(), tc.ErrorIsNil)
}

func (s *manifoldSuite) TestMissingAPICallerName(c *tc.C) {
	s.config.APICallerName = ""
	s.checkNotValid(c, "empty APICallerName not valid")
}

func (s *manifoldSuite) TestMissingLogger(c *tc.C) {
	s.config.Logger = nil
	s.checkNotValid(c, "nil Logger not valid")
}
func (s *manifoldSuite) TestMissingNewWorker(c *tc.C) {
	s.config.NewWorker = nil
	s.checkNotValid(c, "nil NewWorker not valid")
}

func (s *manifoldSuite) TestMissingNewFacade(c *tc.C) {
	s.config.NewUserSecretsFacade = nil
	s.checkNotValid(c, "nil NewUserSecretsFacade not valid")
}

func (s *manifoldSuite) checkNotValid(c *tc.C, expect string) {
	err := s.config.Validate()
	c.Check(err, tc.ErrorMatches, expect)
	c.Check(err, tc.ErrorIs, errors.NotValid)
}

func (s *manifoldSuite) TestStart(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	facade := mocks.NewMockSecretsFacade(ctrl)
	s.config.NewUserSecretsFacade = func(base.APICaller) secretspruner.SecretsFacade {
		return facade
	}

	called := false
	s.config.NewWorker = func(config secretspruner.Config) (worker.Worker, error) {
		called = true
		mc := tc.NewMultiChecker()
		mc.AddExpr(`_.Logger`, tc.NotNil)
		c.Check(config, mc, secretspruner.Config{SecretsFacade: facade})
		return nil, nil
	}
	manifold := secretspruner.Manifold(s.config)
	w, err := manifold.Start(c.Context(), dt.StubGetter(map[string]interface{}{
		"api-caller": struct{ base.APICaller }{&mockAPICaller{}},
	}))
	c.Assert(w, tc.IsNil)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(called, tc.IsTrue)
}

type mockAPICaller struct {
	base.APICaller
}

func (*mockAPICaller) BestFacadeVersion(facade string) int {
	return 1
}
