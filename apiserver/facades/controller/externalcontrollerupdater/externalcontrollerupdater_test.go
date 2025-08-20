// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package externalcontrollerupdater_test

import (
	"testing"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/names/v6"
	"github.com/juju/tc"
	"github.com/juju/worker/v4/workertest"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/apiserver/facades/controller/externalcontrollerupdater"
	"github.com/juju/juju/core/crossmodel"
	"github.com/juju/juju/core/watcher/registry"
	coretesting "github.com/juju/juju/internal/testing"
	"github.com/juju/juju/rpc/params"
)

func TestCrossControllerSuite(t *testing.T) {
	tc.Run(t, &CrossControllerSuite{})
}

type CrossControllerSuite struct {
	coretesting.BaseSuite

	watcherRegistry *registry.Registry
}

func (s *CrossControllerSuite) SetUpTest(c *tc.C) {
	s.BaseSuite.SetUpTest(c)

	var err error
	s.watcherRegistry, err = registry.NewRegistry(clock.WallClock)
	c.Assert(err, tc.ErrorIsNil)
	s.AddCleanup(func(c *tc.C) { workertest.DirtyKill(c, s.watcherRegistry) })
}

func (s *CrossControllerSuite) TestExternalControllerInfo(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	ecService := NewMockExternalControllerService(ctrl)

	ctrlTag, err := names.ParseControllerTag(coretesting.ControllerTag.String())
	c.Assert(err, tc.ErrorIsNil)
	ecService.EXPECT().Controller(gomock.Any(), ctrlTag.Id()).Return(&crossmodel.ControllerInfo{
		ControllerUUID: coretesting.ControllerTag.Id(),
		Alias:          "foo",
		Addrs:          []string{"bar"},
		CACert:         "baz",
	}, nil)

	modelTag, err := names.ParseControllerTag("controller-" + coretesting.ModelTag.Id())
	c.Assert(err, tc.ErrorIsNil)
	ecService.EXPECT().Controller(gomock.Any(), modelTag.Id()).Return(nil, errors.NotFoundf("external controller with UUID deadbeef-0bad-400d-8000-4b1d0d06f00d"))

	api, err := externalcontrollerupdater.NewAPI(ecService, s.watcherRegistry)
	c.Assert(err, tc.ErrorIsNil)
	results, err := api.ExternalControllerInfo(c.Context(), params.Entities{
		Entities: []params.Entity{
			{Tag: coretesting.ControllerTag.String()},
			{Tag: "controller-" + coretesting.ModelTag.Id()},
			{Tag: "machine-42"},
		},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(results, tc.DeepEquals, params.ExternalControllerInfoResults{
		Results: []params.ExternalControllerInfoResult{{
			Result: &params.ExternalControllerInfo{
				ControllerTag: coretesting.ControllerTag.String(),
				Alias:         "foo",
				Addrs:         []string{"bar"},
				CACert:        "baz",
			},
		}, {
			Error: &params.Error{
				Code:    "not found",
				Message: `external controller with UUID deadbeef-0bad-400d-8000-4b1d0d06f00d not found`,
			},
		}, {
			Error: &params.Error{Message: `"machine-42" is not a valid controller tag`},
		}},
	})
}

func (s *CrossControllerSuite) TestSetExternalControllerInfo(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	ecService := NewMockExternalControllerService(ctrl)

	firstControllerTag := coretesting.ControllerTag.String()
	firstControllerTagParsed, err := names.ParseControllerTag(firstControllerTag)
	c.Assert(err, tc.ErrorIsNil)
	secondControllerTag := "controller-" + coretesting.ModelTag.Id()
	secondControllerTagParsed, err := names.ParseControllerTag(secondControllerTag)
	c.Assert(err, tc.ErrorIsNil)

	ecService.EXPECT().UpdateExternalController(gomock.Any(), crossmodel.ControllerInfo{
		ControllerUUID: firstControllerTagParsed.Id(),
		Alias:          "foo",
		Addrs:          []string{"bar"},
		CACert:         "baz",
	})
	ecService.EXPECT().UpdateExternalController(gomock.Any(), crossmodel.ControllerInfo{
		ControllerUUID: secondControllerTagParsed.Id(),
		Alias:          "qux",
		Addrs:          []string{"quux"},
		CACert:         "quuz",
	})

	api, err := externalcontrollerupdater.NewAPI(ecService, s.watcherRegistry)
	c.Assert(err, tc.ErrorIsNil)

	results, err := api.SetExternalControllerInfo(c.Context(), params.SetExternalControllersInfoParams{
		Controllers: []params.SetExternalControllerInfoParams{{
			Info: params.ExternalControllerInfo{
				ControllerTag: firstControllerTag,
				Alias:         "foo",
				Addrs:         []string{"bar"},
				CACert:        "baz",
			},
		}, {
			Info: params.ExternalControllerInfo{
				ControllerTag: secondControllerTag,
				Alias:         "qux",
				Addrs:         []string{"quux"},
				CACert:        "quuz",
			},
		}, {
			Info: params.ExternalControllerInfo{
				ControllerTag: "machine-42",
			},
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(results, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{
			{Error: nil},
			{Error: nil},
			{Error: &params.Error{Message: `"machine-42" is not a valid controller tag`}},
		},
	})
}

func (s *CrossControllerSuite) TestWatchExternalControllers(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	ecService := NewMockExternalControllerService(ctrl)
	mockKeysWatcher := NewMockStringsWatcher(ctrl)
	ecService.EXPECT().Watch(gomock.Any()).Return(mockKeysWatcher, nil)
	changes := make(chan []string, 1)
	mockKeysWatcher.EXPECT().Changes().Return(changes)
	mockKeysWatcher.EXPECT().Kill().AnyTimes()

	done := make(chan struct{})
	mockKeysWatcher.EXPECT().Wait().Return(nil).DoAndReturn(func() error {
		select {
		case <-done:
		case <-c.Context().Done():
			c.Fatalf("timed out waiting for watcher to finish")
		}
		return nil
	})

	api, err := externalcontrollerupdater.NewAPI(ecService, s.watcherRegistry)
	c.Assert(err, tc.ErrorIsNil)

	changes <- []string{"a", "b"} // initial value

	results, err := api.WatchExternalControllers(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(results, tc.DeepEquals, params.StringsWatchResults{
		Results: []params.StringsWatchResult{{
			StringsWatcherId: "w-1",
			Changes:          []string{"a", "b"},
		}},
	})

	w, err := s.watcherRegistry.Get("w-1")
	c.Assert(err, tc.ErrorIsNil)
	c.Check(w, tc.Equals, mockKeysWatcher)

	close(done)
}

func (s *CrossControllerSuite) TestWatchControllerInfoError(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	ecService := NewMockExternalControllerService(ctrl)
	mockKeysWatcher := NewMockStringsWatcher(ctrl)
	ecService.EXPECT().Watch(gomock.Any()).Return(mockKeysWatcher, nil)
	changes := make(chan []string, 1)
	mockKeysWatcher.EXPECT().Changes().Return(changes)
	mockKeysWatcher.EXPECT().Kill().AnyTimes()
	mockKeysWatcher.EXPECT().Wait().Return(errors.New("nope")).AnyTimes()

	close(changes)

	api, err := externalcontrollerupdater.NewAPI(ecService, s.watcherRegistry)
	c.Assert(err, tc.ErrorIsNil)

	results, err := api.WatchExternalControllers(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(results, tc.DeepEquals, params.StringsWatchResults{
		Results: []params.StringsWatchResult{{
			Error: &params.Error{Message: "watching external controllers changes: nope"},
		}},
	})
	_, err = s.watcherRegistry.Get("w-1")
	c.Assert(err, tc.ErrorIs, errors.NotFound)
}
