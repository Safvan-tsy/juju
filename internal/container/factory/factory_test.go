// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package factory_test

import (
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/internal/container"
	"github.com/juju/juju/internal/container/factory"
	"github.com/juju/juju/internal/testing"
)

type factorySuite struct {
	testing.BaseSuite
}

func TestFactorySuite(t *stdtesting.T) { tc.Run(t, &factorySuite{}) }
func (*factorySuite) TestNewContainerManager(c *tc.C) {
	for _, test := range []struct {
		containerType instance.ContainerType
		valid         bool
	}{{
		containerType: instance.LXD,
		valid:         true,
	}, {
		containerType: instance.NONE,
		valid:         false,
	}, {
		containerType: instance.ContainerType("other"),
		valid:         false,
	}} {
		conf := container.ManagerConfig{container.ConfigModelUUID: testing.ModelTag.Id()}
		manager, err := factory.NewContainerManager(test.containerType, conf)
		if test.valid {
			c.Assert(err, tc.ErrorIsNil)
			c.Assert(manager, tc.NotNil)
		} else {
			c.Assert(err, tc.ErrorMatches, `unknown container type: ".*"`)
			c.Assert(manager, tc.IsNil)
		}
	}
}
