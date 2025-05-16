// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider_test

import (
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/caas/kubernetes/provider"
	"github.com/juju/juju/internal/testing"
)

type initSuite struct {
	testing.BaseSuite
}

func TestInitSuite(t *stdtesting.T) { tc.Run(t, &initSuite{}) }
func (s *initSuite) TestLabelSelectorGlobalResourcesLifecycle(c *tc.C) {
	c.Assert(
		provider.CompileLifecycleModelTeardownSelector().String(), tc.DeepEquals,
		`juju-resource-lifecycle notin (persistent)`,
	)
}
