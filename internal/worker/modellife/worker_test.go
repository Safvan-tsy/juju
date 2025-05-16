// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modellife

import (
	"context"
	"sync/atomic"
	stdtesting "testing"
	"time"

	jujuerrors "github.com/juju/errors"
	"github.com/juju/tc"
	"github.com/juju/worker/v4/dependency"
	"github.com/juju/worker/v4/workertest"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/core/life"
	"github.com/juju/juju/core/model"
	modeltesting "github.com/juju/juju/core/model/testing"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/core/watcher/watchertest"
	modelerrors "github.com/juju/juju/domain/model/errors"
	"github.com/juju/juju/internal/errors"
	"github.com/juju/juju/internal/testhelpers"
)

type workerSuite struct {
	testhelpers.IsolationSuite

	modelService *MockModelService

	modelUUID model.UUID
}

func TestWorkerSuite(t *stdtesting.T) { tc.Run(t, &workerSuite{}) }
func (s *workerSuite) TestValidateConfig(c *tc.C) {
	cfg := s.getConfig()
	c.Check(cfg.Validate(), tc.IsNil)

	cfg = s.getConfig()
	cfg.ModelUUID = ""
	c.Check(cfg.Validate(), tc.ErrorIs, jujuerrors.NotValid)

	cfg = s.getConfig()
	cfg.Result = nil
	c.Check(cfg.Validate(), tc.ErrorIs, jujuerrors.NotValid)

	cfg = s.getConfig()
	cfg.ModelService = nil
	c.Check(cfg.Validate(), tc.ErrorIs, jujuerrors.NotValid)
}

func (s *workerSuite) TestStartAlive(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.modelService.EXPECT().GetModelLife(gomock.Any(), s.modelUUID).Return(life.Alive, nil)

	done := make(chan struct{})
	s.modelService.EXPECT().WatchModel(gomock.Any(), s.modelUUID).DoAndReturn(func(ctx context.Context, u model.UUID) (watcher.Watcher[struct{}], error) {
		close(done)
		return watchertest.NewMockNotifyWatcher(make(<-chan struct{})), nil
	})

	w := s.newWorker(c)
	defer workertest.DirtyKill(c, w)

	select {
	case <-done:
	case <-time.After(testhelpers.LongWait):
		c.Fatal("timed out waiting for worker to start")
	}

	c.Assert(w.Check(), tc.IsTrue)

	workertest.CleanKill(c, w)
}

func (s *workerSuite) TestStartDead(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.modelService.EXPECT().GetModelLife(gomock.Any(), s.modelUUID).Return(life.Dead, nil)

	done := make(chan struct{})
	s.modelService.EXPECT().WatchModel(gomock.Any(), s.modelUUID).DoAndReturn(func(ctx context.Context, u model.UUID) (watcher.Watcher[struct{}], error) {
		close(done)
		return watchertest.NewMockNotifyWatcher(make(<-chan struct{})), nil
	})

	w := s.newWorker(c)
	defer workertest.DirtyKill(c, w)

	select {
	case <-done:
	case <-time.After(testhelpers.LongWait):
		c.Fatal("timed out waiting for worker to start")
	}

	c.Assert(w.Check(), tc.IsFalse)

	workertest.CleanKill(c, w)
}

func (s *workerSuite) TestStartError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.modelService.EXPECT().GetModelLife(gomock.Any(), s.modelUUID).Return(life.Alive, modelerrors.NotFound)

	cfg := s.getConfig()

	_, err := NewWorker(c.Context(), cfg)
	c.Assert(err, tc.ErrorIs, modelerrors.NotFound)
}

func (s *workerSuite) TestWatchModelError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.modelService.EXPECT().GetModelLife(gomock.Any(), s.modelUUID).Return(life.Alive, nil)

	done := make(chan struct{})

	s.modelService.EXPECT().WatchModel(gomock.Any(), s.modelUUID).DoAndReturn(func(ctx context.Context, u model.UUID) (watcher.Watcher[struct{}], error) {
		defer close(done)
		return nil, errors.Errorf("boom")
	})

	w := s.newWorker(c)
	defer workertest.DirtyKill(c, w)

	select {
	case <-done:
	case <-time.After(testhelpers.LongWait):
		c.Fatal("timed out waiting for worker to start")
	}

	err := workertest.CheckKilled(c, w)
	c.Assert(err, tc.ErrorMatches, "boom")
}

func (s *workerSuite) TestWatchModelStillAlive(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.modelService.EXPECT().GetModelLife(gomock.Any(), s.modelUUID).Return(life.Alive, nil)

	done := make(chan struct{})

	ch := make(chan struct{})
	s.modelService.EXPECT().WatchModel(gomock.Any(), s.modelUUID).DoAndReturn(func(ctx context.Context, u model.UUID) (watcher.Watcher[struct{}], error) {
		return watchertest.NewMockNotifyWatcher(ch), nil
	})
	s.modelService.EXPECT().GetModelLife(gomock.Any(), s.modelUUID).DoAndReturn(func(ctx context.Context, u model.UUID) (life.Value, error) {
		close(done)
		return life.Alive, nil
	})

	w := s.newWorker(c)
	defer workertest.DirtyKill(c, w)

	select {
	case ch <- struct{}{}:
	case <-time.After(testhelpers.LongWait):
		c.Fatal("timed out waiting for worker to start")
	}

	select {
	case <-done:
	case <-time.After(testhelpers.LongWait):
		c.Fatal("timed out waiting for worker to start")
	}

	c.Assert(w.Check(), tc.IsTrue)

	workertest.CleanKill(c, w)
}

func (s *workerSuite) TestWatchModelTransitionAliveToDying(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.modelService.EXPECT().GetModelLife(gomock.Any(), s.modelUUID).Return(life.Alive, nil)

	done := make(chan struct{})

	ch := make(chan struct{})
	s.modelService.EXPECT().WatchModel(gomock.Any(), s.modelUUID).DoAndReturn(func(ctx context.Context, u model.UUID) (watcher.Watcher[struct{}], error) {
		return watchertest.NewMockNotifyWatcher(ch), nil
	})
	s.modelService.EXPECT().GetModelLife(gomock.Any(), s.modelUUID).DoAndReturn(func(ctx context.Context, u model.UUID) (life.Value, error) {
		close(done)
		return life.Dying, nil
	})

	w := s.newWorker(c)
	defer workertest.DirtyKill(c, w)

	select {
	case ch <- struct{}{}:
	case <-time.After(testhelpers.LongWait):
		c.Fatal("timed out waiting for worker to start")
	}

	select {
	case <-done:
	case <-time.After(testhelpers.LongWait):
		c.Fatal("timed out waiting for worker to start")
	}

	c.Assert(w.Check(), tc.IsTrue)

	err := workertest.CheckKilled(c, w)
	c.Assert(err, tc.ErrorIs, dependency.ErrBounce)
}

func (s *workerSuite) TestWatchModelTransitionDyingToDead(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.modelService.EXPECT().GetModelLife(gomock.Any(), s.modelUUID).Return(life.Dying, nil)

	done := make(chan struct{})

	ch := make(chan struct{}, 2)
	s.modelService.EXPECT().WatchModel(gomock.Any(), s.modelUUID).DoAndReturn(func(ctx context.Context, u model.UUID) (watcher.Watcher[struct{}], error) {
		return watchertest.NewMockNotifyWatcher(ch), nil
	})

	var count int64
	s.modelService.EXPECT().GetModelLife(gomock.Any(), s.modelUUID).DoAndReturn(func(ctx context.Context, u model.UUID) (life.Value, error) {
		if c := atomic.AddInt64(&count, 1); c > 1 {
			close(done)
		}
		return life.Dead, nil
	}).Times(2)

	w := s.newWorker(c)
	defer workertest.DirtyKill(c, w)

	for i := 0; i < 2; i++ {
		select {
		case ch <- struct{}{}:
		case <-time.After(testhelpers.LongWait):
			c.Fatal("timed out waiting for worker to start")
		}
	}

	select {
	case <-done:
	case <-time.After(testhelpers.LongWait):
		c.Fatal("timed out waiting for worker to start")
	}

	c.Assert(w.Check(), tc.IsFalse)

	workertest.CleanKill(c, w)
}

func (s *workerSuite) getConfig() Config {
	return Config{
		ModelUUID:    s.modelUUID,
		ModelService: s.modelService,
		Result:       life.IsAlive,
	}
}

func (s *workerSuite) newWorker(c *tc.C) *Worker {
	cfg := s.getConfig()

	w, err := NewWorker(c.Context(), cfg)
	c.Assert(err, tc.ErrorIsNil)

	return w.(*Worker)
}

func (s *workerSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.modelUUID = modeltesting.GenModelUUID(c)

	s.modelService = NewMockModelService(ctrl)

	return ctrl
}
