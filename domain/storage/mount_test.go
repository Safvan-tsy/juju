// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package storage_test

import (
	"testing"

	"github.com/juju/tc"

	"github.com/juju/juju/domain/storage"
	"github.com/juju/juju/internal/testhelpers"
)

type mountSuite struct {
	testhelpers.IsolationSuite
}

func TestMountSuite(t *testing.T) {
	tc.Run(t, &mountSuite{})
}

func (s *mountSuite) TestMountPointSingleton(c *tc.C) {
	path, err := storage.FilesystemMountPoint("here", 1, "data/0")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(path, tc.Equals, "here")
}

func (s *mountSuite) TestMountPointLocation(c *tc.C) {
	path, err := storage.FilesystemMountPoint("/path/to/here", 2, "data/0")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(path, tc.Equals, "/path/to/here/data/0")
}

func (s *mountSuite) TestMountPointNoLocation(c *tc.C) {
	path, err := storage.FilesystemMountPoint("", 2, "data/0")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(path, tc.Equals, "/var/lib/juju/storage/data/0")
}

func (s *mountSuite) TestBadMountPoint(c *tc.C) {
	_, err := storage.FilesystemMountPoint("/var/lib/juju/storage/here", 1, "data/0")
	c.Assert(err, tc.ErrorMatches, "invalid location .*")
}
