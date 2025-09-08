// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuclient_test

import (
	"os"
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/api/jujuclient"
	"github.com/juju/juju/internal/testing"
)

type BootstrapConfigSuite struct {
	testing.FakeJujuXDGDataHomeSuite
	store jujuclient.BootstrapConfigStore
}

func TestBootstrapConfigSuite(t *stdtesting.T) {
	tc.Run(t, &BootstrapConfigSuite{})
}

func (s *BootstrapConfigSuite) SetUpTest(c *tc.C) {
	s.FakeJujuXDGDataHomeSuite.SetUpTest(c)
	s.store = jujuclient.NewFileClientStore()
	writeTestBootstrapConfigFile(c)
}

func (s *BootstrapConfigSuite) TestBootstrapConfigForControllerNoFile(c *tc.C) {
	err := os.Remove(jujuclient.JujuBootstrapConfigPath())
	c.Assert(err, tc.ErrorIsNil)
	details, err := s.store.BootstrapConfigForController("not-found")
	c.Assert(err, tc.ErrorMatches, "bootstrap config for controller not-found not found")
	c.Assert(details, tc.IsNil)
}

func (s *BootstrapConfigSuite) TestBootstrapConfigForControllerNotFound(c *tc.C) {
	details, err := s.store.BootstrapConfigForController("not-found")
	c.Assert(err, tc.ErrorMatches, "bootstrap config for controller not-found not found")
	c.Assert(details, tc.IsNil)
}

func (s *BootstrapConfigSuite) TestBootstrapConfigForController(c *tc.C) {
	cfg, err := s.store.BootstrapConfigForController("aws-test")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(cfg, tc.NotNil)
	c.Assert(*cfg, tc.DeepEquals, testBootstrapConfig["aws-test"])
}

func (s *BootstrapConfigSuite) TestUpdateBootstrapConfigNewController(c *tc.C) {
	err := s.store.UpdateBootstrapConfig("new-controller", testBootstrapConfig["mallards"])
	c.Assert(err, tc.ErrorIsNil)
	cfg, err := s.store.BootstrapConfigForController("new-controller")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(*cfg, tc.DeepEquals, testBootstrapConfig["mallards"])
}

func (s *BootstrapConfigSuite) TestUpdateBootstrapConfigOverwrites(c *tc.C) {
	err := s.store.UpdateBootstrapConfig("aws-test", testBootstrapConfig["mallards"])
	c.Assert(err, tc.ErrorIsNil)
	cfg, err := s.store.BootstrapConfigForController("aws-test")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(*cfg, tc.DeepEquals, testBootstrapConfig["mallards"])
}
