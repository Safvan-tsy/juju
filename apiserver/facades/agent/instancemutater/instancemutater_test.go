// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package instancemutater_test

import (
	"fmt"
	stdtesting "testing"
	"time"

	"github.com/juju/errors"
	"github.com/juju/names/v6"
	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	apiservererrors "github.com/juju/juju/apiserver/errors"
	facademocks "github.com/juju/juju/apiserver/facade/mocks"
	"github.com/juju/juju/apiserver/facades/agent/instancemutater"
	"github.com/juju/juju/apiserver/facades/agent/instancemutater/mocks"
	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/life"
	"github.com/juju/juju/core/machine"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/status"
	applicationcharm "github.com/juju/juju/domain/application/charm"
	machineerrors "github.com/juju/juju/domain/machine/errors"
	internalcharm "github.com/juju/juju/internal/charm"
	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/internal/testing"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/state"
)

type instanceMutaterAPISuite struct {
	testhelpers.IsolationSuite

	authorizer         *facademocks.MockAuthorizer
	entity             *mocks.MockEntity
	lifer              *mocks.MockLifer
	state              *mocks.MockInstanceMutaterState
	machineService     *mocks.MockMachineService
	applicationService *mocks.MockApplicationService
	modelInfoService   *mocks.MockModelInfoService
	mutatorWatcher     *mocks.MockInstanceMutatorWatcher
	resources          *facademocks.MockResources

	machineTag  names.Tag
	notifyDone  chan struct{}
	stringsDone chan []string
}

func (s *instanceMutaterAPISuite) SetUpTest(c *tc.C) {
	s.IsolationSuite.SetUpTest(c)

	s.machineTag = names.NewMachineTag("0")
	s.notifyDone = make(chan struct{})
	s.stringsDone = make(chan []string)
}

func (s *instanceMutaterAPISuite) setup(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.authorizer = facademocks.NewMockAuthorizer(ctrl)
	s.entity = mocks.NewMockEntity(ctrl)
	s.lifer = mocks.NewMockLifer(ctrl)
	s.state = mocks.NewMockInstanceMutaterState(ctrl)
	s.mutatorWatcher = mocks.NewMockInstanceMutatorWatcher(ctrl)
	s.resources = facademocks.NewMockResources(ctrl)
	s.machineService = mocks.NewMockMachineService(ctrl)
	s.applicationService = mocks.NewMockApplicationService(ctrl)
	s.modelInfoService = mocks.NewMockModelInfoService(ctrl)

	return ctrl
}

func (s *instanceMutaterAPISuite) facadeAPIForScenario(c *tc.C) *instancemutater.InstanceMutaterAPI {
	facade, err := instancemutater.NewTestAPI(c, s.state, s.machineService, s.applicationService, s.modelInfoService, s.mutatorWatcher, s.resources, s.authorizer)
	c.Assert(err, tc.IsNil)
	return facade
}

func (s *instanceMutaterAPISuite) expectLife(machineTag names.Tag) {
	exp := s.authorizer.EXPECT()
	gomock.InOrder(
		exp.AuthController().Return(true),
		exp.AuthMachineAgent().Return(true),
		exp.GetAuthTag().Return(machineTag),
	)
}

func (s *instanceMutaterAPISuite) expectMachine(machineTag names.Tag, machine *mocks.MockMachine) {
	s.state.EXPECT().Machine(machineTag.Id()).Return(machine, nil)
}

func (s *instanceMutaterAPISuite) expectFindMachineError(machineTag names.Tag, err error) {
	s.state.EXPECT().Machine(machineTag.Id()).Return(nil, err)
}

func (s *instanceMutaterAPISuite) expectAuthMachineAgent() {
	s.authorizer.EXPECT().AuthMachineAgent().Return(true)
}

func (s *instanceMutaterAPISuite) assertNotifyStop(c *tc.C) {
	select {
	case <-s.notifyDone:
	case <-time.After(testing.LongWait):
		c.Errorf("timed out waiting for notifications to be consumed")
	}
}

func (s *instanceMutaterAPISuite) assertStringsStop(c *tc.C) {
	select {
	case <-s.stringsDone:
	case <-time.After(testing.LongWait):
		c.Errorf("timed out waiting for notifications to be consumed")
	}
}

type InstanceMutaterAPILifeSuite struct {
	instanceMutaterAPISuite
}

func TestInstanceMutaterAPILifeSuite(t *stdtesting.T) {
	tc.Run(t, &InstanceMutaterAPILifeSuite{})
}
func (s *InstanceMutaterAPILifeSuite) TestLife(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectFindEntity(s.machineTag, entityShim{
		Entity: s.entity,
		Lifer:  s.lifer,
	})
	facade := s.facadeAPIForScenario(c)

	results, err := facade.Life(c.Context(), params.Entities{
		Entities: []params.Entity{{Tag: "machine-0"}},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(results.Results, tc.HasLen, 1)
	c.Assert(results, tc.DeepEquals, params.LifeResults{
		Results: []params.LifeResult{
			{
				Life: life.Alive,
			},
		},
	})
}

func (s *InstanceMutaterAPILifeSuite) TestLifeWithInvalidType(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	facade := s.facadeAPIForScenario(c)

	results, err := facade.Life(c.Context(), params.Entities{
		Entities: []params.Entity{{Tag: "user-0"}},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(results.Results, tc.HasLen, 1)
	c.Assert(results, tc.DeepEquals, params.LifeResults{
		Results: []params.LifeResult{
			{
				Error: &params.Error{
					Message: "permission denied",
					Code:    "unauthorized access",
				},
			},
		},
	})
}

func (s *InstanceMutaterAPILifeSuite) TestLifeWithParentId(c *tc.C) {
	defer s.setup(c).Finish()

	machineTag := names.NewMachineTag("0/lxd/0")

	s.expectAuthMachineAgent()
	s.expectLife(machineTag)
	s.expectFindEntity(machineTag, entityShim{
		Entity: s.entity,
		Lifer:  s.lifer,
	})
	facade := s.facadeAPIForScenario(c)

	results, err := facade.Life(c.Context(), params.Entities{
		Entities: []params.Entity{{Tag: "machine-0-lxd-0"}},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(results.Results, tc.HasLen, 1)
	c.Assert(results, tc.DeepEquals, params.LifeResults{
		Results: []params.LifeResult{
			{
				Life: life.Alive,
			},
		},
	})
}

func (s *InstanceMutaterAPILifeSuite) TestLifeWithInvalidParentId(c *tc.C) {
	defer s.setup(c).Finish()

	machineTag := names.NewMachineTag("0/lxd/0")

	s.expectAuthMachineAgent()
	s.expectLife(machineTag)
	facade := s.facadeAPIForScenario(c)

	results, err := facade.Life(c.Context(), params.Entities{
		Entities: []params.Entity{{Tag: "machine-1-lxd-0"}},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(results.Results, tc.HasLen, 1)
	c.Assert(results, tc.DeepEquals, params.LifeResults{
		Results: []params.LifeResult{
			{
				Error: &params.Error{
					Message: "permission denied",
					Code:    "unauthorized access",
				},
			},
		},
	})
}

func (s *InstanceMutaterAPILifeSuite) expectFindEntity(machineTag names.Tag, entity state.Entity) {
	s.state.EXPECT().FindEntity(machineTag).Return(entity, nil)
	s.lifer.EXPECT().Life().Return(state.Alive)
}

type entityShim struct {
	state.Entity
	state.Lifer
}

type InstanceMutaterAPICharmProfilingInfoSuite struct {
	instanceMutaterAPISuite

	machine     *mocks.MockMachine
	unit        *mocks.MockUnit
	application *mocks.MockApplication
}

func TestInstanceMutaterAPICharmProfilingInfoSuite(t *stdtesting.T) {
	tc.Run(t, &InstanceMutaterAPICharmProfilingInfoSuite{})
}
func (s *InstanceMutaterAPICharmProfilingInfoSuite) setup(c *tc.C) *gomock.Controller {
	ctrl := s.instanceMutaterAPISuite.setup(c)

	s.machine = mocks.NewMockMachine(ctrl)
	s.unit = mocks.NewMockUnit(ctrl)
	s.application = mocks.NewMockApplication(ctrl)

	return ctrl
}

func (s *InstanceMutaterAPICharmProfilingInfoSuite) TestCharmProfilingInfo(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectMachine(s.machineTag, s.machine)
	s.expectUnits(state.Alive)
	s.expectProfileExtraction(c)
	facade := s.facadeAPIForScenario(c)

	s.machine.EXPECT().Id().Return("0")
	s.machineService.EXPECT().GetMachineUUID(gomock.Any(), machine.Name("0")).Return("uuid0", nil)
	s.machineService.EXPECT().AppliedLXDProfileNames(gomock.Any(), machine.UUID("uuid0")).Return([]string{"charm-app-0"}, nil)
	s.machineService.EXPECT().InstanceID(gomock.Any(), machine.UUID("uuid0")).Return("0", nil)

	s.modelInfoService.EXPECT().GetModelInfo(gomock.Any()).Return(model.ModelInfo{
		Name: "foo",
	}, nil)

	results, err := facade.CharmProfilingInfo(c.Context(), params.Entity{Tag: "machine-0"})
	c.Assert(err, tc.IsNil)
	c.Assert(results.Error, tc.IsNil)
	c.Assert(results.InstanceId, tc.Equals, instance.Id("0"))
	c.Assert(results.ModelName, tc.Equals, "foo")
	c.Assert(results.ProfileChanges, tc.HasLen, 1)
	c.Assert(results.CurrentProfiles, tc.HasLen, 1)
	c.Assert(results.ProfileChanges, tc.DeepEquals, []params.ProfileInfoResult{
		{
			ApplicationName: "foo",
			Revision:        0,
			Profile: &params.CharmLXDProfile{
				Config: map[string]string{
					"security.nesting": "true",
				},
				Description: "dummy profile description",
				Devices: map[string]map[string]string{
					"tun": {
						"path": "/dev/net/tun",
					},
				},
			},
		},
	})
	c.Assert(results.CurrentProfiles, tc.DeepEquals, []string{
		"charm-app-0",
	})
}

func (s *InstanceMutaterAPICharmProfilingInfoSuite) TestCharmProfilingInfoWithNoProfile(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectMachine(s.machineTag, s.machine)
	s.expectUnits(state.Alive, state.Alive, state.Dead)
	s.expectProfileExtraction(c)
	s.expectProfileExtractionWithEmpty(c)
	facade := s.facadeAPIForScenario(c)

	s.machine.EXPECT().Id().Return("0")
	s.machineService.EXPECT().GetMachineUUID(gomock.Any(), machine.Name("0")).Return("uuid0", nil)
	s.machineService.EXPECT().AppliedLXDProfileNames(gomock.Any(), machine.UUID("uuid0")).Return([]string{"charm-app-0"}, nil)
	s.machineService.EXPECT().InstanceID(gomock.Any(), machine.UUID("uuid0")).Return("0", nil)

	s.modelInfoService.EXPECT().GetModelInfo(gomock.Any()).Return(model.ModelInfo{
		Name: "foo",
	}, nil)

	results, err := facade.CharmProfilingInfo(c.Context(), params.Entity{Tag: "machine-0"})
	c.Assert(err, tc.IsNil)
	c.Assert(results.Error, tc.IsNil)
	c.Assert(results.InstanceId, tc.Equals, instance.Id("0"))
	c.Assert(results.ModelName, tc.Equals, "foo")
	c.Assert(results.ProfileChanges, tc.HasLen, 2)
	c.Assert(results.CurrentProfiles, tc.HasLen, 1)
	c.Assert(results.ProfileChanges, tc.DeepEquals, []params.ProfileInfoResult{
		{
			ApplicationName: "foo",
			Revision:        0,
			Profile: &params.CharmLXDProfile{
				Config: map[string]string{
					"security.nesting": "true",
				},
				Description: "dummy profile description",
				Devices: map[string]map[string]string{
					"tun": {
						"path": "/dev/net/tun",
					},
				},
			},
		},
		{
			ApplicationName: "foo",
			Revision:        0,
		},
	})
	c.Assert(results.CurrentProfiles, tc.DeepEquals, []string{
		"charm-app-0",
	})
}

func (s *InstanceMutaterAPICharmProfilingInfoSuite) TestCharmProfilingInfoWithInvalidMachine(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectFindMachineError(s.machineTag, errors.New("not found"))
	facade := s.facadeAPIForScenario(c)

	results, err := facade.CharmProfilingInfo(c.Context(), params.Entity{Tag: "machine-0"})
	c.Assert(err, tc.IsNil)
	c.Assert(results.Error, tc.ErrorMatches, "not found")
}

func (s *InstanceMutaterAPICharmProfilingInfoSuite) TestCharmProfilingInfoWithMachineNotProvisioned(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectMachine(s.machineTag, s.machine)
	facade := s.facadeAPIForScenario(c)
	s.machine.EXPECT().Id().Return("0")
	s.machineService.EXPECT().GetMachineUUID(gomock.Any(), machine.Name("0")).Return("uuid0", nil)
	s.machineService.EXPECT().InstanceID(gomock.Any(), machine.UUID("uuid0")).Return("", machineerrors.NotProvisioned)

	results, err := facade.CharmProfilingInfo(c.Context(), params.Entity{Tag: "machine-0"})
	c.Assert(err, tc.IsNil)
	c.Assert(results.Error, tc.ErrorMatches, ".* not provisioned")
	c.Assert(results.InstanceId, tc.Equals, instance.Id(""))
	c.Assert(results.ModelName, tc.Equals, "")
	c.Assert(results.ProfileChanges, tc.HasLen, 0)
	c.Assert(results.CurrentProfiles, tc.HasLen, 0)
}

func (s *InstanceMutaterAPICharmProfilingInfoSuite) expectUnits(lives ...state.Life) {
	machineExp := s.machine.EXPECT()
	units := make([]instancemutater.Unit, len(lives))
	for i := 0; i < len(lives); i++ {
		units[i] = s.unit
		s.unit.EXPECT().Life().Return(lives[i])
		if lives[i] == state.Dead {
			s.unit.EXPECT().Name().Return("foo")
		}
	}
	machineExp.Units().Return(units, nil)
}

func (s *InstanceMutaterAPICharmProfilingInfoSuite) expectProfileExtraction(c *tc.C) {
	appExp := s.application.EXPECT()
	stateExp := s.state.EXPECT()
	unitExp := s.unit.EXPECT()

	unitExp.ApplicationName().Return("foo")
	stateExp.Application("foo").Return(s.application, nil)
	chURLStr := "ch:app-0"
	appExp.CharmURL().Return(&chURLStr)
	s.assertCharmWithLXDProfile(c, chURLStr)

}

func (s *InstanceMutaterAPICharmProfilingInfoSuite) assertCharmWithLXDProfile(c *tc.C, chURLStr string) {
	curl, err := internalcharm.ParseURL(chURLStr)
	c.Assert(err, tc.ErrorIsNil)
	source, err := applicationcharm.ParseCharmSchema(internalcharm.Schema(curl.Schema))
	c.Assert(err, tc.ErrorIsNil)

	s.applicationService.EXPECT().GetCharmLXDProfile(gomock.Any(), applicationcharm.CharmLocator{
		Source:   source,
		Name:     curl.Name,
		Revision: curl.Revision,
	}).
		Return(internalcharm.LXDProfile{
			Config: map[string]string{
				"security.nesting": "true",
			},
			Description: "dummy profile description",
			Devices: map[string]map[string]string{
				"tun": {
					"path": "/dev/net/tun",
				},
			},
		}, 0, nil)
}

func (s *InstanceMutaterAPICharmProfilingInfoSuite) expectProfileExtractionWithEmpty(c *tc.C) {
	appExp := s.application.EXPECT()
	stateExp := s.state.EXPECT()
	unitExp := s.unit.EXPECT()

	unitExp.ApplicationName().Return("foo")
	stateExp.Application("foo").Return(s.application, nil)
	chURLStr := "ch:app-0"
	appExp.CharmURL().Return(&chURLStr)
	s.assertCharmWithoutLXDProfile(c, chURLStr)
}

func (s *InstanceMutaterAPICharmProfilingInfoSuite) assertCharmWithoutLXDProfile(c *tc.C, chURLStr string) {
	curl, err := internalcharm.ParseURL(chURLStr)
	c.Assert(err, tc.ErrorIsNil)
	source, err := applicationcharm.ParseCharmSchema(internalcharm.Schema(curl.Schema))
	c.Assert(err, tc.ErrorIsNil)

	s.applicationService.EXPECT().GetCharmLXDProfile(gomock.Any(), applicationcharm.CharmLocator{
		Source:   source,
		Name:     curl.Name,
		Revision: curl.Revision,
	}).
		Return(internalcharm.LXDProfile{}, 0, nil)
}

type InstanceMutaterAPISetCharmProfilesSuite struct {
	instanceMutaterAPISuite

	machine *mocks.MockMachine
}

func TestInstanceMutaterAPISetCharmProfilesSuite(t *stdtesting.T) {
	tc.Run(t, &InstanceMutaterAPISetCharmProfilesSuite{})
}
func (s *InstanceMutaterAPISetCharmProfilesSuite) setup(c *tc.C) *gomock.Controller {
	ctrl := s.instanceMutaterAPISuite.setup(c)

	s.machine = mocks.NewMockMachine(ctrl)

	return ctrl
}

func (s *InstanceMutaterAPISetCharmProfilesSuite) TestSetCharmProfiles(c *tc.C) {
	defer s.setup(c).Finish()

	profiles := []string{"unit-foo-0"}

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	facade := s.facadeAPIForScenario(c)

	s.machineService.EXPECT().GetMachineUUID(gomock.Any(), machine.Name("0")).Return("uuid0", nil)
	s.machineService.EXPECT().SetAppliedLXDProfileNames(gomock.Any(), machine.UUID("uuid0"), profiles).Return(nil)

	results, err := facade.SetCharmProfiles(c.Context(), params.SetProfileArgs{
		Args: []params.SetProfileArg{
			{
				Entity:   params.Entity{Tag: "machine-0"},
				Profiles: profiles,
			},
		},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(results.Results, tc.HasLen, 1)
	c.Assert(results.Results, tc.DeepEquals, []params.ErrorResult{{}})
}

func (s *InstanceMutaterAPISetCharmProfilesSuite) TestSetCharmProfilesWithError(c *tc.C) {
	defer s.setup(c).Finish()

	profiles := []string{"unit-foo-0"}

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	facade := s.facadeAPIForScenario(c)

	s.machineService.EXPECT().GetMachineUUID(gomock.Any(), machine.Name("0")).Return("uuid0", nil).Times(2)
	s.machineService.EXPECT().SetAppliedLXDProfileNames(gomock.Any(), machine.UUID("uuid0"), profiles).Return(nil)
	s.machineService.EXPECT().SetAppliedLXDProfileNames(gomock.Any(), machine.UUID("uuid0"), profiles).Return(errors.New("Failure"))

	results, err := facade.SetCharmProfiles(c.Context(), params.SetProfileArgs{
		Args: []params.SetProfileArg{
			{
				Entity:   params.Entity{Tag: "machine-0"},
				Profiles: profiles,
			},
			{
				Entity:   params.Entity{Tag: "machine-0"},
				Profiles: profiles,
			},
		},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(results.Results, tc.HasLen, 2)
	c.Assert(results.Results, tc.DeepEquals, []params.ErrorResult{
		{},
		{
			Error: &params.Error{
				Message: "Failure",
			},
		},
	})
}

type InstanceMutaterAPISetModificationStatusSuite struct {
	instanceMutaterAPISuite

	machine *mocks.MockMachine
}

func TestInstanceMutaterAPISetModificationStatusSuite(t *stdtesting.T) {
	tc.Run(t, &InstanceMutaterAPISetModificationStatusSuite{})
}
func (s *InstanceMutaterAPISetModificationStatusSuite) setup(c *tc.C) *gomock.Controller {
	ctrl := s.instanceMutaterAPISuite.setup(c)

	s.machine = mocks.NewMockMachine(ctrl)

	return ctrl
}

func (s *InstanceMutaterAPISetModificationStatusSuite) TestSetModificationStatusProfiles(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectMachine(s.machineTag, s.machine)
	s.expectSetModificationStatus(status.Applied, "applied", nil)
	facade := s.facadeAPIForScenario(c)

	result, err := facade.SetModificationStatus(c.Context(), params.SetStatus{
		Entities: []params.EntityStatusArgs{
			{Tag: "machine-0", Status: "applied", Info: "applied", Data: nil},
		},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(result, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{
			{},
		},
	})
}

func (s *InstanceMutaterAPISetModificationStatusSuite) TestSetModificationStatusProfilesWithError(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectMachine(s.machineTag, s.machine)
	s.expectSetModificationStatus(status.Applied, "applied", errors.New("failed"))
	facade := s.facadeAPIForScenario(c)

	result, err := facade.SetModificationStatus(c.Context(), params.SetStatus{
		Entities: []params.EntityStatusArgs{
			{Tag: "machine-0", Status: "applied", Info: "applied", Data: nil},
		},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(result, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{
			{Error: &params.Error{Message: "failed"}},
		},
	})
}

func (s *InstanceMutaterAPISetModificationStatusSuite) expectSetModificationStatus(st status.Status, message string, err error) {
	now := time.Now()

	sExp := s.state.EXPECT()
	sExp.ControllerTimestamp().Return(&now, nil)

	mExp := s.machine.EXPECT()
	mExp.SetModificationStatus(status.StatusInfo{
		Status:  st,
		Message: message,
		Data:    nil,
		Since:   &now,
	}).Return(err)
}

type InstanceMutaterAPIWatchMachinesSuite struct {
	instanceMutaterAPISuite

	machine *mocks.MockMachine
	watcher *mocks.MockStringsWatcher
}

func TestInstanceMutaterAPIWatchMachinesSuite(t *stdtesting.T) {
	tc.Run(t, &InstanceMutaterAPIWatchMachinesSuite{})
}
func (s *InstanceMutaterAPIWatchMachinesSuite) setup(c *tc.C) *gomock.Controller {
	ctrl := s.instanceMutaterAPISuite.setup(c)

	s.machine = mocks.NewMockMachine(ctrl)
	s.watcher = mocks.NewMockStringsWatcher(ctrl)

	return ctrl
}

func (s *InstanceMutaterAPIWatchMachinesSuite) TestWatchModelMachines(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectAuthController()
	s.expectWatchModelMachinesWithNotify(1)
	facade := s.facadeAPIForScenario(c)

	result, err := facade.WatchModelMachines(c.Context())
	c.Assert(err, tc.IsNil)
	c.Assert(result, tc.DeepEquals, params.StringsWatchResult{
		StringsWatcherId: "1",
		Changes:          []string{"0"},
	})
	s.assertNotifyStop(c)
}

func (s *InstanceMutaterAPIWatchMachinesSuite) TestWatchModelMachinesWithClosedChannel(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectAuthController()
	s.expectWatchModelMachinesWithClosedChannel()
	facade := s.facadeAPIForScenario(c)

	_, err := facade.WatchModelMachines(c.Context())
	c.Assert(err, tc.ErrorMatches, "cannot obtain initial model machines")
}

func (s *InstanceMutaterAPIWatchMachinesSuite) TestWatchMachines(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectAuthController()
	s.expectWatchMachinesWithNotify(1)
	facade := s.facadeAPIForScenario(c)

	result, err := facade.WatchMachines(c.Context())
	c.Assert(err, tc.IsNil)
	c.Assert(result, tc.DeepEquals, params.StringsWatchResult{
		StringsWatcherId: "1",
		Changes:          []string{"0"},
	})
	s.assertNotifyStop(c)
}

func (s *InstanceMutaterAPIWatchMachinesSuite) TestWatchMachinesWithClosedChannel(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectAuthController()
	s.expectWatchMachinesWithClosedChannel()
	facade := s.facadeAPIForScenario(c)

	_, err := facade.WatchMachines(c.Context())
	c.Assert(err, tc.ErrorMatches, "cannot obtain initial model machines")
}

func (s *InstanceMutaterAPIWatchMachinesSuite) expectAuthController() {
	s.authorizer.EXPECT().AuthController().Return(true)
}

func (s *InstanceMutaterAPIWatchMachinesSuite) expectWatchMachinesWithNotify(times int) {
	ch := make(chan []string)

	go func() {
		for i := 0; i < times; i++ {
			ch <- []string{fmt.Sprintf("%d", i)}
		}
		close(s.notifyDone)
	}()

	s.state.EXPECT().WatchMachines().Return(s.watcher)
	s.watcher.EXPECT().Changes().Return(ch)
	s.resources.EXPECT().Register(s.watcher).Return("1")
}

func (s *InstanceMutaterAPIWatchMachinesSuite) expectWatchModelMachinesWithNotify(times int) {
	ch := make(chan []string)

	go func() {
		for i := 0; i < times; i++ {
			ch <- []string{fmt.Sprintf("%d", i)}
		}
		close(s.notifyDone)
	}()

	s.state.EXPECT().WatchModelMachines().Return(s.watcher)
	s.watcher.EXPECT().Changes().Return(ch)
	s.resources.EXPECT().Register(s.watcher).Return("1")
}

func (s *InstanceMutaterAPIWatchMachinesSuite) expectWatchMachinesWithClosedChannel() {
	ch := make(chan []string)
	close(ch)

	s.state.EXPECT().WatchMachines().Return(s.watcher)
	s.watcher.EXPECT().Changes().Return(ch)
}

func (s *InstanceMutaterAPIWatchMachinesSuite) expectWatchModelMachinesWithClosedChannel() {
	ch := make(chan []string)
	close(ch)

	s.state.EXPECT().WatchModelMachines().Return(s.watcher)
	s.watcher.EXPECT().Changes().Return(ch)
}

type InstanceMutaterAPIWatchLXDProfileVerificationNeededSuite struct {
	instanceMutaterAPISuite

	machine *mocks.MockMachine
	watcher *mocks.MockNotifyWatcher
}

func TestInstanceMutaterAPIWatchLXDProfileVerificationNeededSuite(t *stdtesting.T) {
	tc.Run(t, &InstanceMutaterAPIWatchLXDProfileVerificationNeededSuite{})
}
func (s *InstanceMutaterAPIWatchLXDProfileVerificationNeededSuite) setup(c *tc.C) *gomock.Controller {
	ctrl := s.instanceMutaterAPISuite.setup(c)

	s.machine = mocks.NewMockMachine(ctrl)
	s.watcher = mocks.NewMockNotifyWatcher(ctrl)

	return ctrl
}

func (s *InstanceMutaterAPIWatchLXDProfileVerificationNeededSuite) TestWatchLXDProfileVerificationNeeded(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectWatchLXDProfileVerificationNeededWithNotify(c, 1)
	facade := s.facadeAPIForScenario(c)

	result, err := facade.WatchLXDProfileVerificationNeeded(c.Context(), params.Entities{
		Entities: []params.Entity{{Tag: s.machineTag.String()}},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(result, tc.DeepEquals, params.NotifyWatchResults{
		Results: []params.NotifyWatchResult{{
			NotifyWatcherId: "1",
		}},
	})
	s.assertNotifyStop(c)
}

func (s *InstanceMutaterAPIWatchLXDProfileVerificationNeededSuite) TestWatchLXDProfileVerificationNeededWithInvalidTag(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	facade := s.facadeAPIForScenario(c)

	result, err := facade.WatchLXDProfileVerificationNeeded(c.Context(), params.Entities{
		Entities: []params.Entity{{Tag: names.NewUserTag("bob@local").String()}},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(result, tc.DeepEquals, params.NotifyWatchResults{
		Results: []params.NotifyWatchResult{{
			Error: apiservererrors.ServerError(apiservererrors.ErrPerm),
		}},
	})
}

func (s *InstanceMutaterAPIWatchLXDProfileVerificationNeededSuite) TestWatchLXDProfileVerificationNeededWithClosedChannel(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectWatchLXDProfileVerificationNeededWithClosedChannel(c)
	facade := s.facadeAPIForScenario(c)

	result, err := facade.WatchLXDProfileVerificationNeeded(c.Context(), params.Entities{
		Entities: []params.Entity{{Tag: s.machineTag.String()}},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(result, tc.DeepEquals, params.NotifyWatchResults{
		Results: []params.NotifyWatchResult{{
			Error: apiservererrors.ServerError(errors.New("cannot obtain initial machine watch application LXD profiles")),
		}},
	})
}

func (s *InstanceMutaterAPIWatchLXDProfileVerificationNeededSuite) TestWatchLXDProfileVerificationNeededWithManualMachine(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectWatchLXDProfileVerificationNeededWithManualMachine(c)
	facade := s.facadeAPIForScenario(c)

	result, err := facade.WatchLXDProfileVerificationNeeded(c.Context(), params.Entities{
		Entities: []params.Entity{{Tag: s.machineTag.String()}},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(result, tc.DeepEquals, params.NotifyWatchResults{
		Results: []params.NotifyWatchResult{{
			Error: apiservererrors.ServerError(errors.NotSupportedf("watching lxd profiles on manual machines")),
		}},
	})
}

func (s *InstanceMutaterAPIWatchLXDProfileVerificationNeededSuite) TestWatchLXDProfileVerificationNeededModelCacheError(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectWatchLXDProfileVerificationNeededError(c)
	facade := s.facadeAPIForScenario(c)

	result, err := facade.WatchLXDProfileVerificationNeeded(c.Context(), params.Entities{
		Entities: []params.Entity{{Tag: s.machineTag.String()}},
	})
	c.Assert(err, tc.IsNil)
	c.Assert(result, tc.DeepEquals, params.NotifyWatchResults{
		Results: []params.NotifyWatchResult{{
			Error: apiservererrors.ServerError(errors.New("watcher error")),
		}},
	})
}

func (s *InstanceMutaterAPIWatchLXDProfileVerificationNeededSuite) expectWatchLXDProfileVerificationNeededWithNotify(c *tc.C, times int) {
	ch := make(chan struct{})

	go func() {
		for i := 0; i < times; i++ {
			ch <- struct{}{}
		}
		close(s.notifyDone)
	}()

	s.state.EXPECT().Machine(s.machineTag.Id()).Return(s.machine, nil)
	s.machine.EXPECT().IsManual().Return(false, nil)
	s.mutatorWatcher.EXPECT().WatchLXDProfileVerificationForMachine(gomock.Any(), s.machine, gomock.Any()).Return(s.watcher, nil)
	s.watcher.EXPECT().Changes().Return(ch)
	s.resources.EXPECT().Register(s.watcher).Return("1")
}

func (s *InstanceMutaterAPIWatchLXDProfileVerificationNeededSuite) expectWatchLXDProfileVerificationNeededWithClosedChannel(c *tc.C) {
	ch := make(chan struct{})
	close(ch)

	s.state.EXPECT().Machine(s.machineTag.Id()).Return(s.machine, nil)
	s.machine.EXPECT().IsManual().Return(false, nil)
	s.mutatorWatcher.EXPECT().WatchLXDProfileVerificationForMachine(gomock.Any(), s.machine, gomock.Any()).Return(s.watcher, nil)
	s.watcher.EXPECT().Changes().Return(ch)
}

func (s *InstanceMutaterAPIWatchLXDProfileVerificationNeededSuite) expectWatchLXDProfileVerificationNeededError(c *tc.C) {
	s.state.EXPECT().Machine(s.machineTag.Id()).Return(s.machine, nil)
	s.machine.EXPECT().IsManual().Return(false, nil)
	s.mutatorWatcher.EXPECT().WatchLXDProfileVerificationForMachine(gomock.Any(), s.machine, gomock.Any()).Return(s.watcher, errors.New("watcher error"))
}

func (s *InstanceMutaterAPIWatchLXDProfileVerificationNeededSuite) expectWatchLXDProfileVerificationNeededWithManualMachine(c *tc.C) {
	ch := make(chan struct{})
	close(ch)

	s.state.EXPECT().Machine(s.machineTag.Id()).Return(s.machine, nil)
	s.machine.EXPECT().IsManual().Return(true, nil)
	s.mutatorWatcher.EXPECT().WatchLXDProfileVerificationForMachine(gomock.Any(), s.machine, gomock.Any()).Times(0)
}

type InstanceMutaterAPIWatchContainersSuite struct {
	instanceMutaterAPISuite

	machine *mocks.MockMachine
	watcher *mocks.MockStringsWatcher
}

func TestInstanceMutaterAPIWatchContainersSuite(t *stdtesting.T) {
	tc.Run(t, &InstanceMutaterAPIWatchContainersSuite{})
}
func (s *InstanceMutaterAPIWatchContainersSuite) setup(c *tc.C) *gomock.Controller {
	ctrl := s.instanceMutaterAPISuite.setup(c)

	s.machine = mocks.NewMockMachine(ctrl)
	s.watcher = mocks.NewMockStringsWatcher(ctrl)

	return ctrl
}

func (s *InstanceMutaterAPIWatchContainersSuite) TestWatchContainers(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectWatchContainersWithNotify(1)
	facade := s.facadeAPIForScenario(c)

	result, err := facade.WatchContainers(c.Context(), params.Entity{Tag: s.machineTag.String()})
	c.Assert(err, tc.IsNil)
	c.Assert(result, tc.DeepEquals, params.StringsWatchResult{
		StringsWatcherId: "1",
		Changes:          []string{"0"},
	})
	s.assertStringsStop(c)
}

func (s *InstanceMutaterAPIWatchContainersSuite) TestWatchContainersWithInvalidTag(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	facade := s.facadeAPIForScenario(c)

	result, err := facade.WatchContainers(c.Context(), params.Entity{Tag: names.NewUserTag("bob@local").String()})
	c.Logf("%#v", err)
	c.Assert(err, tc.ErrorMatches, "\"user-bob\" is not a valid machine tag")
	c.Assert(result, tc.DeepEquals, params.StringsWatchResult{})
}

func (s *InstanceMutaterAPIWatchContainersSuite) TestWatchContainersWithClosedChannel(c *tc.C) {
	defer s.setup(c).Finish()

	s.expectAuthMachineAgent()
	s.expectLife(s.machineTag)
	s.expectWatchContainersWithClosedChannel()
	facade := s.facadeAPIForScenario(c)

	result, err := facade.WatchContainers(c.Context(), params.Entity{Tag: s.machineTag.String()})
	c.Assert(err, tc.ErrorMatches, "cannot obtain initial machine containers")
	c.Assert(result, tc.DeepEquals, params.StringsWatchResult{})
}

func (s *InstanceMutaterAPIWatchContainersSuite) expectWatchContainersWithNotify(times int) {
	ch := make(chan []string)

	go func() {
		for i := 0; i < times; i++ {
			ch <- []string{fmt.Sprintf("%d", i)}
		}
		close(s.stringsDone)
	}()

	s.state.EXPECT().Machine(s.machineTag.Id()).Return(s.machine, nil)
	s.machine.EXPECT().WatchContainers(instance.LXD).Return(s.watcher)
	s.watcher.EXPECT().Changes().Return(ch)
	s.resources.EXPECT().Register(s.watcher).Return("1")
}

func (s *InstanceMutaterAPIWatchContainersSuite) expectWatchContainersWithClosedChannel() {
	ch := make(chan []string)
	close(ch)

	s.state.EXPECT().Machine(s.machineTag.Id()).Return(s.machine, nil)
	s.machine.EXPECT().WatchContainers(instance.LXD).Return(s.watcher)
	s.watcher.EXPECT().Changes().Return(ch)
}
