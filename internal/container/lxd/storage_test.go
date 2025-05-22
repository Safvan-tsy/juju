// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package lxd_test

import (
	stdtesting "testing"

	lxdapi "github.com/canonical/lxd/shared/api"
	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/internal/container/lxd"
	lxdtesting "github.com/juju/juju/internal/container/lxd/testing"
)

type storageSuite struct {
	lxdtesting.BaseSuite
}

func TestStorageSuite(t *stdtesting.T) {
	tc.Run(t, &storageSuite{})
}

func defaultProfileWithDisk() *lxdapi.Profile {
	return &lxdapi.Profile{
		Name: "default",
		Devices: map[string]map[string]string{
			"root": {
				"type": "disk",
				"path": "/",
				"pool": "default",
			},
		},
	}
}

func (s *storageSuite) TestStorageIsSupported(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "storage")

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, tc.ErrorIsNil)

	c.Check(jujuSvr.StorageSupported(), tc.IsTrue)
}

func (s *storageSuite) TestStorageNotSupported(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "network")

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, tc.ErrorIsNil)

	c.Check(jujuSvr.StorageSupported(), tc.IsFalse)
}

func (s *storageSuite) TestCreatePool(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "storage")

	cfg := map[string]string{"size": "1024MB"}

	req := lxdapi.StoragePoolsPost{
		Name:   "new-pool",
		Driver: "dir",
		StoragePoolPut: lxdapi.StoragePoolPut{
			Config: cfg,
		},
	}
	cSvr.EXPECT().CreateStoragePool(req).Return(nil)

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, tc.ErrorIsNil)

	err = jujuSvr.CreatePool("new-pool", "dir", cfg)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *storageSuite) TestCreateVolume(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "storage")

	cfg := map[string]string{"size": "1024MB"}

	req := lxdapi.StorageVolumesPost{
		Name: "volume",
		Type: "custom",
		StorageVolumePut: lxdapi.StorageVolumePut{
			Config: cfg,
		},
	}
	cSvr.EXPECT().CreateStoragePoolVolume("default-pool", req).Return(nil)

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, tc.ErrorIsNil)

	err = jujuSvr.CreateVolume("default-pool", "volume", cfg)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *storageSuite) TestEnsureDefaultStorageDevicePresent(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "storage")

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, tc.ErrorIsNil)

	c.Assert(jujuSvr.EnsureDefaultStorage(defaultProfileWithDisk(), ""), tc.ErrorIsNil)
}

func (s *storageSuite) TestEnsureDefaultStoragePoolExistsDeviceCreated(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "storage")

	profile := defaultProfileWithDisk()
	gomock.InOrder(
		cSvr.EXPECT().GetStoragePoolNames().Return([]string{"default"}, nil),
		cSvr.EXPECT().UpdateProfile("default", profile.Writable(), lxdtesting.ETag).Return(nil),
	)

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, tc.ErrorIsNil)

	profile.Devices = nil
	c.Assert(jujuSvr.EnsureDefaultStorage(profile, lxdtesting.ETag), tc.ErrorIsNil)
}

func (s *storageSuite) TestEnsureDefaultStorageNonDefaultPoolExistsDeviceCreated(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "storage")

	profile := defaultProfileWithDisk()
	profile.Devices["root"]["pool"] = "custom"
	gomock.InOrder(
		cSvr.EXPECT().GetStoragePoolNames().Return([]string{"custom"}, nil),
		cSvr.EXPECT().UpdateProfile("default", profile.Writable(), lxdtesting.ETag).Return(nil),
	)

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, tc.ErrorIsNil)

	profile.Devices = nil
	c.Assert(jujuSvr.EnsureDefaultStorage(profile, lxdtesting.ETag), tc.ErrorIsNil)
}

func (s *storageSuite) TestEnsureDefaultStoragePoolAndDeviceCreated(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "storage")

	profile := defaultProfileWithDisk()
	req := lxdapi.StoragePoolsPost{
		Name:   "default",
		Driver: "dir",
	}
	gomock.InOrder(
		cSvr.EXPECT().GetStoragePoolNames().Return(nil, nil),
		cSvr.EXPECT().CreateStoragePool(req).Return(nil),
		cSvr.EXPECT().UpdateProfile("default", profile.Writable(), lxdtesting.ETag).Return(nil),
	)

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, tc.ErrorIsNil)

	profile.Devices = nil
	c.Assert(jujuSvr.EnsureDefaultStorage(profile, lxdtesting.ETag), tc.ErrorIsNil)
}
