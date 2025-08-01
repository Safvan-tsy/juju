// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modellife

import (
	"context"
	"testing"

	"github.com/juju/tc"
	"github.com/juju/worker/v4"
	"github.com/juju/worker/v4/dependency"
	dt "github.com/juju/worker/v4/dependency/testing"
	"github.com/juju/worker/v4/workertest"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/core/errors"
	"github.com/juju/juju/internal/testhelpers"
)

type ManifoldSuite struct {
	testhelpers.IsolationSuite

	modelService *MockModelService
}

func TestManifoldSuite(t *testing.T) {
	defer goleak.VerifyNone(t)
	tc.Run(t, &ManifoldSuite{})
}

func (s *ManifoldSuite) TestValidateConfig(c *tc.C) {
	cfg := s.getConfig()
	c.Check(cfg.Validate(), tc.ErrorIsNil)

	cfg = s.getConfig()
	cfg.DomainServicesName = ""
	c.Check(cfg.Validate(), tc.ErrorIs, errors.NotValid)

	cfg = s.getConfig()
	cfg.NewWorker = nil
	c.Check(cfg.Validate(), tc.ErrorIs, errors.NotValid)

	cfg = s.getConfig()
	cfg.ModelUUID = ""
	c.Check(cfg.Validate(), tc.ErrorIs, errors.NotValid)

	cfg = s.getConfig()
	cfg.GetModelService = nil
	c.Check(cfg.Validate(), tc.ErrorIs, errors.NotValid)
}

var expectedInputs = []string{"domainservices"}

func (s *ManifoldSuite) TestInputs(c *tc.C) {
	c.Assert(s.newManifold(c).Inputs, tc.SameContents, expectedInputs)
}

func (s *ManifoldSuite) TestMissingInputs(c *tc.C) {
	defer s.setupMocks(c).Finish()

	for _, input := range expectedInputs {
		getter := s.newGetter(c, map[string]any{
			input: dependency.ErrMissing,
		})
		_, err := s.newManifold(c).Start(c.Context(), getter)
		c.Assert(err, tc.ErrorIs, dependency.ErrMissing)
	}
}

func (s *ManifoldSuite) TestStart(c *tc.C) {
	w, err := s.newManifold(c).Start(c.Context(), s.newGetter(c, map[string]any{
		"domainservices": s.modelService,
	}))
	c.Assert(err, tc.ErrorIsNil)

	workertest.CheckAlive(c, w)

	workertest.CleanKill(c, w)
}

func (s *ManifoldSuite) newManifold(c *tc.C) dependency.Manifold {
	manifold := Manifold(s.getConfig())
	return manifold
}

func (s *ManifoldSuite) getConfig() ManifoldConfig {
	return ManifoldConfig{
		DomainServicesName: "domainservices",
		ModelUUID:          "model-uuid",
		NewWorker: func(ctx context.Context, c Config) (worker.Worker, error) {
			return workertest.NewErrorWorker(nil), nil
		},
		GetModelService: func(d dependency.Getter, name string) (ModelService, error) {
			var modelService ModelService
			if err := d.Get(name, &modelService); err != nil {
				return nil, err
			}
			return modelService, nil
		},
	}
}

func (s *ManifoldSuite) newGetter(c *tc.C, overlay map[string]any) dependency.Getter {
	resources := map[string]any{
		"domainservices": s.modelService,
	}
	for k, v := range overlay {
		resources[k] = v
	}
	return dt.StubGetter(resources)
}

func (s *ManifoldSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.modelService = NewMockModelService(ctrl)

	return ctrl
}
