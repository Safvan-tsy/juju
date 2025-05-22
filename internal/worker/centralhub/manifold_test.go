// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package centralhub_test

import (
	"testing"

	"github.com/juju/errors"
	"github.com/juju/pubsub/v2"
	"github.com/juju/tc"
	"github.com/juju/worker/v4/dependency"
	dt "github.com/juju/worker/v4/dependency/testing"
	"github.com/juju/worker/v4/workertest"

	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/internal/worker/centralhub"
)

type ManifoldSuite struct {
	testhelpers.IsolationSuite
	hub    *pubsub.StructuredHub
	config centralhub.ManifoldConfig
}

func TestManifoldSuite(t *testing.T) {
	tc.Run(t, &ManifoldSuite{})
}

func (s *ManifoldSuite) SetUpTest(c *tc.C) {
	s.IsolationSuite.SetUpTest(c)
	s.hub = pubsub.NewStructuredHub(nil)
	s.config = centralhub.ManifoldConfig{
		StateConfigWatcherName: "state-config-watcher",
		Hub:                    s.hub,
	}
}

func (s *ManifoldSuite) manifold() dependency.Manifold {
	return centralhub.Manifold(s.config)
}

func (s *ManifoldSuite) TestInputs(c *tc.C) {
	c.Check(s.manifold().Inputs, tc.DeepEquals, []string{"state-config-watcher"})
}

func (s *ManifoldSuite) TestStateConfigWatcherMissing(c *tc.C) {
	getter := dt.StubGetter(map[string]interface{}{
		"state-config-watcher": dependency.ErrMissing,
	})

	worker, err := s.manifold().Start(c.Context(), getter)
	c.Check(worker, tc.IsNil)
	c.Check(errors.Cause(err), tc.Equals, dependency.ErrMissing)
}

func (s *ManifoldSuite) TestStateConfigWatcherNotAStateServer(c *tc.C) {
	getter := dt.StubGetter(map[string]interface{}{
		"state-config-watcher": false,
	})

	worker, err := s.manifold().Start(c.Context(), getter)
	c.Check(worker, tc.IsNil)
	c.Check(errors.Cause(err), tc.Equals, dependency.ErrMissing)
}

func (s *ManifoldSuite) TestMissingHub(c *tc.C) {
	s.config.Hub = nil
	getter := dt.StubGetter(map[string]interface{}{
		"state-config-watcher": true,
	})

	worker, err := s.manifold().Start(c.Context(), getter)
	c.Check(worker, tc.IsNil)
	c.Check(errors.Cause(err), tc.ErrorIs, errors.NotValid)
}

func (s *ManifoldSuite) TestHubOutput(c *tc.C) {
	getter := dt.StubGetter(map[string]interface{}{
		"state-config-watcher": true,
	})

	manifold := s.manifold()
	worker, err := manifold.Start(c.Context(), getter)
	c.Check(err, tc.ErrorIsNil)
	c.Assert(worker, tc.NotNil)
	defer workertest.CleanKill(c, worker)

	var hub *pubsub.StructuredHub
	err = manifold.Output(worker, &hub)
	c.Check(err, tc.ErrorIsNil)
	c.Check(hub, tc.Equals, s.hub)
}
