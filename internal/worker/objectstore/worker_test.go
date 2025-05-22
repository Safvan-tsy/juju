// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package objectstore

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	stdtesting "testing"
	"time"

	"github.com/juju/tc"
	"github.com/juju/worker/v4"
	"github.com/juju/worker/v4/workertest"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/core/objectstore"
	internalobjectstore "github.com/juju/juju/internal/objectstore"
	"github.com/juju/juju/internal/testing"
	"github.com/juju/juju/internal/uuid"
)

type workerSuite struct {
	baseSuite

	states                     chan string
	trackedObjectStore         *MockTrackedObjectStore
	controllerMetadataService  *MockMetadataService
	modelMetadataServiceGetter *MockMetadataServiceGetter
	modelClaimGetter           *MockModelClaimGetter
	modelMetadataService       *MockMetadataService
	called                     int64
}

func TestWorkerSuite(t *stdtesting.T) {
	defer goleak.VerifyNone(t)
	tc.Run(t, &workerSuite{})
}

func (s *workerSuite) TestKilledGetObjectStoreErrDying(c *tc.C) {
	defer s.setupMocks(c).Finish()

	w := s.newWorker(c)
	defer workertest.DirtyKill(c, w)

	s.ensureStartup(c)

	w.Kill()

	worker := w.(*objectStoreWorker)
	_, err := worker.GetObjectStore(c.Context(), "foo")
	c.Assert(err, tc.ErrorIs, objectstore.ErrObjectStoreDying)
}

func (s *workerSuite) TestGetObjectStore(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.expectClock()

	w := s.newWorker(c)
	defer workertest.CleanKill(c, w)

	s.ensureStartup(c)

	done := make(chan struct{})
	s.trackedObjectStore.EXPECT().Kill().AnyTimes()
	s.trackedObjectStore.EXPECT().Wait().DoAndReturn(func() error {
		<-done
		return nil
	}).AnyTimes()

	worker := w.(*objectStoreWorker)
	objectStore, err := worker.GetObjectStore(c.Context(), "foo")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(objectStore, tc.NotNil)

	close(done)

	workertest.CleanKill(c, w)
}

func (s *workerSuite) TestGetObjectStoreIsCached(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.expectClock()

	w := s.newWorker(c)
	defer workertest.CleanKill(c, w)

	s.ensureStartup(c)

	done := make(chan struct{})
	s.trackedObjectStore.EXPECT().Kill().AnyTimes()
	s.trackedObjectStore.EXPECT().Wait().DoAndReturn(func() error {
		<-done
		return nil
	}).AnyTimes()

	worker := w.(*objectStoreWorker)
	for i := 0; i < 10; i++ {

		_, err := worker.GetObjectStore(c.Context(), "foo")
		c.Assert(err, tc.ErrorIsNil)
	}

	close(done)

	workertest.CleanKill(c, w)

	c.Assert(atomic.LoadInt64(&s.called), tc.Equals, int64(1))
}

func (s *workerSuite) TestGetObjectStoreIsNotCachedForDifferentNamespaces(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.expectClock()

	w := s.newWorker(c)
	defer workertest.CleanKill(c, w)

	s.ensureStartup(c)

	done := make(chan struct{})
	s.trackedObjectStore.EXPECT().Kill().AnyTimes()
	s.trackedObjectStore.EXPECT().Wait().DoAndReturn(func() error {
		<-done
		return nil
	}).AnyTimes()

	worker := w.(*objectStoreWorker)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("anything-%d", i)

		_, err := worker.GetObjectStore(c.Context(), name)
		c.Assert(err, tc.ErrorIsNil)
	}

	close(done)

	workertest.CleanKill(c, w)

	c.Assert(atomic.LoadInt64(&s.called), tc.Equals, int64(10))
}

func (s *workerSuite) TestGetObjectStoreConcurrently(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.expectClock()

	w := s.newWorker(c)
	defer workertest.CleanKill(c, w)

	s.ensureStartup(c)

	done := make(chan struct{})
	s.trackedObjectStore.EXPECT().Kill().AnyTimes()
	s.trackedObjectStore.EXPECT().Wait().DoAndReturn(func() error {
		<-done
		return nil
	}).AnyTimes()

	var wg sync.WaitGroup
	wg.Add(10)

	worker := w.(*objectStoreWorker)
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer wg.Done()

			name := fmt.Sprintf("anything-%d", i)

			_, err := worker.GetObjectStore(c.Context(), name)
			c.Assert(err, tc.ErrorIsNil)
		}(i)
	}

	assertWait(c, wg.Wait)
	c.Assert(atomic.LoadInt64(&s.called), tc.Equals, int64(10))

	close(done)

	workertest.CleanKill(c, w)
}

func (s *workerSuite) newWorker(c *tc.C) worker.Worker {
	w, err := newWorker(WorkerConfig{
		Clock:        s.clock,
		Logger:       s.logger,
		TracerGetter: &stubTracerGetter{},
		S3Client:     s.s3Client,
		NewObjectStoreWorker: func(context.Context, objectstore.BackendType, string, ...internalobjectstore.Option) (internalobjectstore.TrackedObjectStore, error) {
			atomic.AddInt64(&s.called, 1)
			return s.trackedObjectStore, nil
		},
		ControllerMetadataService:  s.controllerMetadataService,
		ModelMetadataServiceGetter: s.modelMetadataServiceGetter,
		ModelClaimGetter:           s.modelClaimGetter,
		RootDir:                    c.MkDir(),
		RootBucket:                 uuid.MustNewUUID().String(),
	}, s.states)
	c.Assert(err, tc.ErrorIsNil)
	return w
}

func (s *workerSuite) setupMocks(c *tc.C) *gomock.Controller {
	// Ensure we buffer the channel, this is because we might miss the
	// event if we're too quick at starting up.
	s.states = make(chan string, 1)
	atomic.StoreInt64(&s.called, 0)

	ctrl := s.baseSuite.setupMocks(c)

	s.trackedObjectStore = NewMockTrackedObjectStore(ctrl)
	s.controllerMetadataService = NewMockMetadataService(ctrl)
	s.modelMetadataService = NewMockMetadataService(ctrl)

	s.modelMetadataServiceGetter = NewMockMetadataServiceGetter(ctrl)
	s.modelMetadataServiceGetter.EXPECT().ForModelUUID(gomock.Any()).Return(s.modelMetadataService).AnyTimes()

	s.modelClaimGetter = NewMockModelClaimGetter(ctrl)
	s.modelClaimGetter.EXPECT().ForModelUUID(gomock.Any()).Return(s.claimer, nil).AnyTimes()

	return ctrl
}

func (s *workerSuite) ensureStartup(c *tc.C) {
	select {
	case state := <-s.states:
		c.Assert(state, tc.Equals, stateStarted)
	case <-time.After(testing.ShortWait * 10):
		c.Fatalf("timed out waiting for startup")
	}
}

func assertWait(c *tc.C, wait func()) {
	done := make(chan struct{})

	go func() {
		defer close(done)
		wait()
	}()

	select {
	case <-done:
	case <-time.After(testing.LongWait):
		c.Fatalf("timed out waiting")
	}
}
