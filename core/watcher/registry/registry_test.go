// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package registry

import (
	"fmt"
	"sync"
	stdtesting "testing"
	"time"

	"github.com/juju/tc"
	"github.com/juju/worker/v4"
	"github.com/juju/worker/v4/workertest"
	"go.uber.org/mock/gomock"

	coreerrors "github.com/juju/juju/core/errors"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/internal/testing"
)

type registrySuite struct {
	testhelpers.IsolationSuite

	clock *MockClock
}

func TestRegistrySuite(t *stdtesting.T) { tc.Run(t, &registrySuite{}) }
func (s *registrySuite) TestRegisterCount(c *tc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()

	s.expectClock()

	reg := s.newRegistry(c)
	defer workertest.DirtyKill(c, reg)

	c.Check(reg.Count(), tc.Equals, 0)

	workertest.CheckKill(c, reg)
}

func (s *registrySuite) TestRegisterGetCount(c *tc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()

	s.expectClock()

	reg := s.newRegistry(c)
	defer workertest.DirtyKill(c, reg)

	for i := 0; i < 10; i++ {
		w := s.expectWatcher(c, ctrl, reg.catacomb.Dying())

		id, err := reg.Register(w)
		c.Assert(err, tc.ErrorIsNil)

		w1, err := reg.Get(id)
		c.Assert(err, tc.ErrorIsNil)
		c.Check(w1, tc.Equals, w)
		c.Check(reg.Count(), tc.Equals, i+1)
	}

	workertest.CheckKill(c, reg)

	c.Check(reg.Count(), tc.Equals, 0)
}

func (s *registrySuite) TestRegisterNamedGetCount(c *tc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()

	s.expectClock()

	reg := s.newRegistry(c)
	defer workertest.DirtyKill(c, reg)

	for i := 0; i < 10; i++ {
		w := s.expectWatcher(c, ctrl, reg.catacomb.Dying())

		id := fmt.Sprintf("id-%d", i)
		err := reg.RegisterNamed(id, w)
		c.Assert(err, tc.ErrorIsNil)

		w1, err := reg.Get(id)
		c.Assert(err, tc.ErrorIsNil)
		c.Check(w1, tc.Equals, w)
		c.Check(reg.Count(), tc.Equals, i+1)
	}

	workertest.CheckKill(c, reg)

	c.Check(reg.Count(), tc.Equals, 0)
}

func (s *registrySuite) TestRegisterNamedRepeatedError(c *tc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()

	s.expectClock()

	reg := s.newRegistry(c)
	defer workertest.DirtyKill(c, reg)

	w := s.expectWatcher(c, ctrl, reg.catacomb.Dying())

	err := reg.RegisterNamed("foo", w)
	c.Assert(err, tc.ErrorIsNil)

	err = reg.RegisterNamed("foo", w)
	c.Assert(err, tc.ErrorMatches, `worker "foo" already exists`)
	c.Assert(err, tc.ErrorIs, coreerrors.AlreadyExists)

	workertest.CheckKill(c, reg)
}

func (s *registrySuite) TestRegisterNamedIntegerName(c *tc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()

	s.expectClock()

	reg := s.newRegistry(c)
	defer workertest.DirtyKill(c, reg)

	w := NewMockWorker(ctrl)

	err := reg.RegisterNamed("0", w)
	c.Assert(err, tc.ErrorMatches, `namespace "0" not valid`)
	c.Assert(err, tc.ErrorIs, coreerrors.NotValid)

	workertest.CheckKill(c, reg)
}

func (s *registrySuite) TestRegisterStop(c *tc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()

	s.expectClock()

	reg := s.newRegistry(c)
	defer workertest.DirtyKill(c, reg)

	done := make(chan struct{})
	w := NewMockWorker(ctrl)
	w.EXPECT().Kill().DoAndReturn(func() {
		close(done)
	})
	w.EXPECT().Wait().DoAndReturn(func() error {
		select {
		case <-done:
		case <-time.After(testing.LongWait):
			c.Fatalf("timed out waiting for worker to finish")
		}

		return nil
	}).MinTimes(1)

	err := reg.RegisterNamed("foo", w)
	c.Assert(err, tc.ErrorIsNil)

	err = reg.Stop("foo")
	c.Assert(err, tc.ErrorIsNil)

	c.Check(reg.Count(), tc.Equals, 0)

	workertest.CheckKill(c, reg)
}

func (s *registrySuite) TestConcurrency(c *tc.C) {
	ctrl := s.setupMocks(c)
	defer ctrl.Finish()

	s.expectClock()

	// This test is designed to cause the race detector
	// to fail if the locking is not done correctly.
	reg := s.newRegistry(c)
	defer workertest.DirtyKill(c, reg)

	var wg sync.WaitGroup
	start := func(f func()) {
		wg.Add(1)
		go func() {
			f()
			wg.Done()
		}()
	}
	reg.Register(s.expectSimpleWatcher(ctrl))
	start(func() {
		reg.Register(s.expectSimpleWatcher(ctrl))
	})
	start(func() {
		reg.RegisterNamed("named", s.expectSimpleWatcher(ctrl))
	})
	start(func() {
		reg.Stop("1")
	})
	start(func() {
		reg.Count()
	})
	start(func() {
		reg.Kill()
	})
	start(func() {
		reg.Get("2")
	})
	start(func() {
		reg.Get("named")
	})
	wg.Wait()
	workertest.CheckKill(c, reg)
}

func (s *registrySuite) newRegistry(c *tc.C) *Registry {
	reg, err := NewRegistry(s.clock, WithLogger(loggertesting.WrapCheckLog(c)))
	c.Assert(err, tc.ErrorIsNil)
	return reg
}

func (s *registrySuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.clock = NewMockClock(ctrl)

	return ctrl
}

func (s *registrySuite) expectClock() {
	s.clock.EXPECT().Now().AnyTimes().Return(time.Now())
}

func (s *registrySuite) expectWatcher(c *tc.C, ctrl *gomock.Controller, done <-chan struct{}) worker.Worker {
	w := NewMockWorker(ctrl)
	w.EXPECT().Kill().AnyTimes()
	w.EXPECT().Wait().DoAndReturn(func() error {
		select {
		case <-done:
		case <-time.After(testing.LongWait):
			c.Fatalf("timed out waiting for worker to finish")
		}

		return nil
	}).MinTimes(1)
	return w
}

func (s *registrySuite) expectSimpleWatcher(ctrl *gomock.Controller) worker.Worker {
	w := NewMockWorker(ctrl)
	w.EXPECT().Kill().AnyTimes()
	w.EXPECT().Wait().DoAndReturn(func() error {
		<-time.After(testing.ShortWait)
		return nil
	}).AnyTimes()
	return w
}
