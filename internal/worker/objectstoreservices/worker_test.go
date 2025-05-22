// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package objectstoreservices

import (
	"testing"

	"github.com/juju/errors"
	"github.com/juju/tc"
	"github.com/juju/worker/v4"
	"github.com/juju/worker/v4/workertest"

	"github.com/juju/juju/core/changestream"
	"github.com/juju/juju/core/logger"
	coremodel "github.com/juju/juju/core/model"
	"github.com/juju/juju/internal/services"
)

type workerSuite struct {
	baseSuite
}

func TestWorkerSuite(t *testing.T) {
	tc.Run(t, &workerSuite{})
}

func (s *workerSuite) TestValidateConfig(c *tc.C) {
	defer s.setupMocks(c).Finish()

	cfg := s.getConfig()
	c.Check(cfg.Validate(), tc.ErrorIsNil)

	cfg = s.getConfig()
	cfg.Logger = nil
	c.Check(cfg.Validate(), tc.ErrorIs, errors.NotValid)

	cfg = s.getConfig()
	cfg.DBGetter = nil
	c.Check(cfg.Validate(), tc.ErrorIs, errors.NotValid)

	cfg = s.getConfig()
	cfg.NewObjectStoreServices = nil
	c.Check(cfg.Validate(), tc.ErrorIs, errors.NotValid)

	cfg = s.getConfig()
	cfg.NewObjectStoreServicesGetter = nil
	c.Check(cfg.Validate(), tc.ErrorIs, errors.NotValid)
}

func (s *workerSuite) getConfig() Config {
	return Config{
		DBGetter: s.dbGetter,
		Logger:   s.logger,
		NewObjectStoreServices: func(coremodel.UUID, changestream.WatchableDBGetter, logger.Logger) services.ObjectStoreServices {
			return s.objectStoreServices
		},
		NewObjectStoreServicesGetter: func(ObjectStoreServicesFn, changestream.WatchableDBGetter, logger.Logger) services.ObjectStoreServicesGetter {
			return s.objectStoreServicesGetter
		},
	}
}

func (s *workerSuite) TestWorkerServicesGetter(c *tc.C) {
	defer s.setupMocks(c).Finish()

	w := s.newWorker(c)
	defer workertest.CleanKill(c, w)

	srvFact, ok := w.(*servicesWorker)
	c.Assert(ok, tc.IsTrue, tc.Commentf("worker does not implement servicesWorker"))

	getter := srvFact.ServicesGetter()
	c.Assert(getter, tc.NotNil)

	workertest.CleanKill(c, w)
}

func (s *workerSuite) newWorker(c *tc.C) worker.Worker {
	w, err := NewWorker(s.getConfig())
	c.Assert(err, tc.ErrorIsNil)
	return w
}
