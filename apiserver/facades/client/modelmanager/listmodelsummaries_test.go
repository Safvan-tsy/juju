// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modelmanager_test

import (
	stdtesting "testing"

	"github.com/juju/tc"

	_ "github.com/juju/juju/internal/provider/azure"
	_ "github.com/juju/juju/internal/provider/ec2"
	_ "github.com/juju/juju/internal/provider/maas"
	_ "github.com/juju/juju/internal/provider/openstack"
)

type ListModelsWithInfoSuite struct{}

func TestListModelsWithInfoSuite(t *stdtesting.T) {
	tc.Run(t, &ListModelsWithInfoSuite{})
}

func (s *ListModelsWithInfoSuite) TestStub(c *tc.C) {
	c.Skip(`skipping test (tlm): Missing tests for the following cases.
	- Happy path list test for a user.
	- Happy path list test for all models.
	- Permission denied test for list model summaries.
	- List model summaries for invalid user (should probably cover a user that doesn't exist).
	  This was a tag test when originally constructed to make sure the tag is valid.
	- Test no models for user.
	- Test no models on the controller.
`)
}
