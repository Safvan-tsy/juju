// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package storageprovisioner

import (
	"context"
	"testing"

	"github.com/juju/errors"
	"github.com/juju/names/v6"
	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/apiserver/common"
	facademocks "github.com/juju/juju/apiserver/facade/mocks"
	"github.com/juju/juju/core/life"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/state"
)

type caasProvisionerSuite struct {
	testhelpers.IsolationSuite

	api *StorageProvisionerAPIv4

	storageBackend       *MockStorageBackend
	applicationService   *MockApplicationService
	machineService       *MockMachineService
	filesystemAttachment *MockFilesystemAttachment
	volumeAttachment     *MockVolumeAttachment
	lifer                *MockLifer
	resources            *MockResources
	watcherRegistry      *facademocks.MockWatcherRegistry
}

func TestCaasProvisionerSuite(t *testing.T) {
	tc.Run(t, &caasProvisionerSuite{})
}

func (s *caasProvisionerSuite) TestRemoveVolumeAttachment(c *tc.C) {
	defer s.setupMocks(c).Finish()

	// It is expected that the detachment of mariadb has been remove prior.

	s.storageBackend.EXPECT().RemoveVolumeAttachment(names.NewUnitTag("mariadb/0"), names.NewVolumeTag("0"), false).Return(errors.Errorf(`removing attachment of volume 0 from unit mariadb/0: volume attachment is not dying`))
	s.storageBackend.EXPECT().RemoveVolumeAttachment(names.NewUnitTag("mariadb/0"), names.NewVolumeTag("1"), false).Return(nil)
	s.storageBackend.EXPECT().RemoveVolumeAttachment(names.NewUnitTag("mysql/2"), names.NewVolumeTag("4"), false).Return(errors.NotFoundf(`removing attachment of volume 4 from unit mysql/2: volume "4" on "unit mysql/2"`))
	s.storageBackend.EXPECT().RemoveVolumeAttachment(names.NewUnitTag("mariadb/0"), names.NewVolumeTag("42"), false).Return(errors.NotFoundf(`removing attachment of volume 42 from unit mariadb/0: volume "42" on "unit mariadb/0"`))

	results, err := s.api.RemoveAttachment(c.Context(), params.MachineStorageIds{
		Ids: []params.MachineStorageId{{
			MachineTag:    "unit-mariadb-0",
			AttachmentTag: "volume-0",
		}, {
			MachineTag:    "unit-mariadb-0",
			AttachmentTag: "volume-1",
		}, {
			MachineTag:    "unit-mysql-2",
			AttachmentTag: "volume-4",
		}, {
			MachineTag:    "unit-mariadb-0",
			AttachmentTag: "volume-42",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(results, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{
			{Error: &params.Error{Message: "removing attachment of volume 0 from unit mariadb/0: volume attachment is not dying"}},
			{Error: nil},
			{Error: &params.Error{Message: `removing attachment of volume 4 from unit mysql/2: volume "4" on "unit mysql/2" not found`, Code: "not found"}},
			{Error: &params.Error{Message: `removing attachment of volume 42 from unit mariadb/0: volume "42" on "unit mariadb/0" not found`, Code: "not found"}},
		},
	})
}

func (s *caasProvisionerSuite) TestRemoveFilesystemAttachments(c *tc.C) {
	defer s.setupMocks(c).Finish()

	// It is expected that the detachment of mariadb has been remove prior.

	s.storageBackend.EXPECT().RemoveFilesystemAttachment(names.NewUnitTag("mariadb/0"), names.NewFilesystemTag("0"), false).Return(errors.Errorf(`removing attachment of filesystem 0 from unit mariadb/0: filesystem attachment is not dying`))
	s.storageBackend.EXPECT().RemoveFilesystemAttachment(names.NewUnitTag("mariadb/0"), names.NewFilesystemTag("1"), false).Return(nil)
	s.storageBackend.EXPECT().RemoveFilesystemAttachment(names.NewUnitTag("mysql/2"), names.NewFilesystemTag("4"), false).Return(errors.NotFoundf(`removing attachment of filesystem 4 from unit mysql/2: filesystem "4" on "unit mysql/2"`))
	s.storageBackend.EXPECT().RemoveFilesystemAttachment(names.NewUnitTag("mariadb/0"), names.NewFilesystemTag("42"), false).Return(errors.NotFoundf(`removing attachment of filesystem 42 from unit mariadb/0: filesystem "42" on "unit mariadb/0"`))

	results, err := s.api.RemoveAttachment(c.Context(), params.MachineStorageIds{
		Ids: []params.MachineStorageId{{
			MachineTag:    "unit-mariadb-0",
			AttachmentTag: "filesystem-0",
		}, {
			MachineTag:    "unit-mariadb-0",
			AttachmentTag: "filesystem-1",
		}, {
			MachineTag:    "unit-mysql-2",
			AttachmentTag: "filesystem-4",
		}, {
			MachineTag:    "unit-mariadb-0",
			AttachmentTag: "filesystem-42",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(results, tc.DeepEquals, params.ErrorResults{
		Results: []params.ErrorResult{
			{Error: &params.Error{Message: "removing attachment of filesystem 0 from unit mariadb/0: filesystem attachment is not dying"}},
			{Error: nil},
			{Error: &params.Error{Message: `removing attachment of filesystem 4 from unit mysql/2: filesystem "4" on "unit mysql/2" not found`, Code: "not found"}},
			{Error: &params.Error{Message: `removing attachment of filesystem 42 from unit mariadb/0: filesystem "42" on "unit mariadb/0" not found`, Code: "not found"}},
		},
	})
}

func (s *caasProvisionerSuite) TestFilesystemAttachmentLife(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.storageBackend.EXPECT().FilesystemAttachment(names.NewUnitTag("mariadb/0"), names.NewFilesystemTag("0")).Return(s.filesystemAttachment, nil)
	s.filesystemAttachment.EXPECT().Life().Return(state.Alive)

	s.storageBackend.EXPECT().FilesystemAttachment(names.NewUnitTag("mariadb/0"), names.NewFilesystemTag("1")).Return(s.filesystemAttachment, nil)
	s.filesystemAttachment.EXPECT().Life().Return(state.Alive)

	s.storageBackend.EXPECT().FilesystemAttachment(names.NewUnitTag("mariadb/0"), names.NewFilesystemTag("42")).Return(s.filesystemAttachment, errors.NotFoundf(`filesystem "42" on "unit mariadb/0"`))

	results, err := s.api.AttachmentLife(c.Context(), params.MachineStorageIds{
		Ids: []params.MachineStorageId{{
			MachineTag:    "unit-mariadb-0",
			AttachmentTag: "filesystem-0",
		}, {
			MachineTag:    "unit-mariadb-0",
			AttachmentTag: "filesystem-1",
		}, {
			MachineTag:    "unit-mariadb-0",
			AttachmentTag: "filesystem-42",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(results, tc.DeepEquals, params.LifeResults{
		Results: []params.LifeResult{
			{Life: life.Alive},
			{Life: life.Alive},
			{Error: &params.Error{Message: `filesystem "42" on "unit mariadb/0" not found`, Code: "not found"}},
		},
	})
}

func (s *caasProvisionerSuite) TestVolumeAttachmentLife(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.storageBackend.EXPECT().VolumeAttachment(names.NewUnitTag("mariadb/0"), names.NewVolumeTag("0")).Return(s.volumeAttachment, nil)
	s.volumeAttachment.EXPECT().Life().Return(state.Alive)

	s.storageBackend.EXPECT().VolumeAttachment(names.NewUnitTag("mariadb/0"), names.NewVolumeTag("1")).Return(s.volumeAttachment, nil)
	s.volumeAttachment.EXPECT().Life().Return(state.Alive)

	s.storageBackend.EXPECT().VolumeAttachment(names.NewUnitTag("mariadb/0"), names.NewVolumeTag("42")).Return(s.volumeAttachment, errors.NotFoundf(`volume "42" on "unit mariadb/0"`))

	results, err := s.api.AttachmentLife(c.Context(), params.MachineStorageIds{
		Ids: []params.MachineStorageId{{
			MachineTag:    "unit-mariadb-0",
			AttachmentTag: "volume-0",
		}, {
			MachineTag:    "unit-mariadb-0",
			AttachmentTag: "volume-1",
		}, {
			MachineTag:    "unit-mariadb-0",
			AttachmentTag: "volume-42",
		}},
	})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(results, tc.DeepEquals, params.LifeResults{
		Results: []params.LifeResult{
			{Life: life.Alive},
			{Life: life.Alive},
			{Error: &params.Error{Message: `volume "42" on "unit mariadb/0" not found`, Code: "not found"}},
		},
	})
}

func (s *caasProvisionerSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.storageBackend = NewMockStorageBackend(ctrl)
	s.filesystemAttachment = NewMockFilesystemAttachment(ctrl)
	s.volumeAttachment = NewMockVolumeAttachment(ctrl)
	s.lifer = NewMockLifer(ctrl)
	s.resources = NewMockResources(ctrl)
	s.applicationService = NewMockApplicationService(ctrl)
	s.watcherRegistry = facademocks.NewMockWatcherRegistry(ctrl)
	s.applicationService = NewMockApplicationService(ctrl)
	s.machineService = NewMockMachineService(ctrl)

	s.api = &StorageProvisionerAPIv4{
		watcherRegistry: s.watcherRegistry,
		LifeGetter: common.NewLifeGetter(s.applicationService, s.machineService, func(context.Context) (common.AuthFunc, error) {
			return func(names.Tag) bool {
				return true
			}, nil
		}, loggertesting.WrapCheckLog(c)),
		sb: s.storageBackend,
		getAttachmentAuthFunc: func(context.Context) (func(names.Tag, names.Tag) bool, error) {
			return func(names.Tag, names.Tag) bool { return true }, nil
		},
		applicationService: s.applicationService,
	}

	return ctrl
}
