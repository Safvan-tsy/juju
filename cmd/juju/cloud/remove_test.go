// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package cloud_test

import (
	"context"
	"os"
	stdtesting "testing"

	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	"github.com/juju/juju/api/jujuclient"
	jujucloud "github.com/juju/juju/cloud"
	"github.com/juju/juju/cmd/juju/cloud"
	"github.com/juju/juju/cmd/juju/cloud/mocks"
	"github.com/juju/juju/environs"
	environmocks "github.com/juju/juju/environs/testing"
	"github.com/juju/juju/internal/cmd"
	"github.com/juju/juju/internal/cmd/cmdtesting"
	"github.com/juju/juju/internal/testing"
	"github.com/juju/juju/juju/osenv"
)

// This is required since our test provider needs to have the capabilty to
// provider built-in clouds. To do so, it must implement the optional interface
// CloudDetector as well as CloudEnvironProvider from environs
type mockBuiltinEnvironProvider struct {
	*environmocks.MockCloudEnvironProvider
	*environmocks.MockCloudDetector
}

type removeSuite struct {
	testing.FakeJujuXDGDataHomeSuite
	store *jujuclient.MemStore

	api          *mocks.MockRemoveCloudAPI
	testProvider *mockBuiltinEnvironProvider
}

func TestRemoveSuite(t *stdtesting.T) {
	tc.Run(t, &removeSuite{})
}

func (s *removeSuite) SetUpTest(c *tc.C) {
	s.FakeJujuXDGDataHomeSuite.SetUpTest(c)
	store := jujuclient.NewMemStore()
	store.Controllers["mycontroller"] = jujuclient.ControllerDetails{}
	store.CurrentControllerName = "mycontroller"
	s.store = store
}

func (s *removeSuite) setup(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.api = mocks.NewMockRemoveCloudAPI(ctrl)
	s.testProvider = &mockBuiltinEnvironProvider{
		environmocks.NewMockCloudEnvironProvider(ctrl),
		environmocks.NewMockCloudDetector(ctrl),
	}
	return ctrl
}

func (s *removeSuite) TestRemoveBadArgs(c *tc.C) {
	command := cloud.NewRemoveCloudCommand()
	_, err := cmdtesting.RunCommand(c, command, "--client")
	c.Assert(err, tc.ErrorMatches, "Usage: juju remove-cloud <cloud name>")
	_, err = cmdtesting.RunCommand(c, command, "cloud", "extra", "--client")
	c.Assert(err, tc.ErrorMatches, `unrecognized args: \["extra"\]`)
}

func (s *removeSuite) TestRemoveNotFound(c *tc.C) {
	command := cloud.NewRemoveCloudCommandForTest(s.store, nil)
	ctx, err := cmdtesting.RunCommand(c, command, "fnord", "--client")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(cmdtesting.Stderr(ctx), tc.Equals, "No cloud called \"fnord\" exists on this client\n")
}

func (s *removeSuite) createTestCloudData(c *tc.C) {
	err := os.WriteFile(osenv.JujuXDGDataHomePath("public-clouds.yaml"), []byte(`
clouds:
  prodstack:
    type: openstack
    auth-types: [userpass, access-key]
    endpoint: http://prodstack
  prodstack2:
    type: openstack
    auth-types: [userpass, access-key]
    endpoint: http://prodstack2
`[1:]), 0600)
	c.Assert(err, tc.ErrorIsNil)

	err = os.WriteFile(osenv.JujuXDGDataHomePath("credentials.yaml"), []byte(`
credentials:
  prodstack2:
    cred-name:
      auth-type: userpass
      username: user
      password: pass
`[1:]), 0600)
	c.Assert(err, tc.ErrorIsNil)

	err = os.WriteFile(osenv.JujuXDGDataHomePath("clouds.yaml"), []byte(`
clouds:
  homestack:
    type: openstack
    auth-types: [userpass, access-key]
    endpoint: http://homestack
  homestack2:
    type: openstack
    auth-types: [userpass, access-key]
    endpoint: http://homestack2
`[1:]), 0600)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *removeSuite) TestRemoveCloudLocal(c *tc.C) {
	defer s.setup(c).Finish()

	command := cloud.NewRemoveCloudCommandForTest(
		s.store,
		func(ctx context.Context) (cloud.RemoveCloudAPI, error) {
			c.Fail()
			return s.api, nil
		})
	s.createTestCloudData(c)
	assertPersonalClouds(c, "homestack", "homestack2")
	ctx, err := cmdtesting.RunCommand(c, command, "homestack", "--client")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(cmdtesting.Stderr(ctx), tc.Equals, "Removed details of cloud \"homestack\" from this client\n")
	assertPersonalClouds(c, "homestack2")
}

func (s *removeSuite) TestRemoveCloudNoControllers(c *tc.C) {
	defer s.setup(c).Finish()

	s.store.Controllers = nil
	command := cloud.NewRemoveCloudCommandForTest(
		s.store,
		func(ctx context.Context) (cloud.RemoveCloudAPI, error) {
			c.Fail()
			return s.api, nil
		})
	s.createTestCloudData(c)
	assertPersonalClouds(c, "homestack", "homestack2")
	ctx, err := cmdtesting.RunCommand(c, command, "homestack", "--client")
	c.Assert(err, tc.ErrorIsNil)
	assertPersonalClouds(c, "homestack2")
	c.Assert(cmdtesting.Stdout(ctx), tc.Equals, ``)
	c.Assert(cmdtesting.Stderr(ctx), tc.Matches, "Removed details of cloud \"homestack\" from this client\n")
}

func (s *removeSuite) TestRemoveCloudControllerControllerOnly(c *tc.C) {
	defer s.setup(c).Finish()

	command := cloud.NewRemoveCloudCommandForTest(
		s.store,
		func(ctx context.Context) (cloud.RemoveCloudAPI, error) {
			return s.api, nil
		})
	s.createTestCloudData(c)

	s.api.EXPECT().RemoveCloud(gomock.Any(), "homestack").Return(nil)
	s.api.EXPECT().Close().Return(nil)
	ctx, err := cmdtesting.RunCommand(c, command, "homestack", "-c", "mycontroller")

	c.Assert(err, tc.ErrorIsNil)
	assertPersonalClouds(c, "homestack", "homestack2")
	c.Assert(command.ControllerName, tc.Equals, "mycontroller")
	c.Assert(cmdtesting.Stderr(ctx), tc.Equals, "Removed details of cloud \"homestack\" from controller \"mycontroller\"\n")
}

func (s *removeSuite) TestRemoveCloudBoth(c *tc.C) {
	defer s.setup(c).Finish()

	command := cloud.NewRemoveCloudCommandForTest(
		s.store,
		func(ctx context.Context) (cloud.RemoveCloudAPI, error) {
			return s.api, nil
		})
	s.createTestCloudData(c)

	s.api.EXPECT().RemoveCloud(gomock.Any(), "homestack").Return(nil)
	s.api.EXPECT().Close().Return(nil)
	ctx, err := cmdtesting.RunCommand(c, command, "homestack", "-c", "mycontroller", "--client")

	c.Assert(err, tc.ErrorIsNil)
	assertPersonalClouds(c, "homestack2")
	c.Assert(command.ControllerName, tc.Equals, "mycontroller")
	c.Assert(cmdtesting.Stderr(ctx), tc.Equals,
		"Removed details of cloud \"homestack\" from this client\n"+
			"Removed details of cloud \"homestack\" from controller \"mycontroller\"\n")
}

func (s *removeSuite) TestCannotRemovePublicCloud(c *tc.C) {
	s.createTestCloudData(c)
	ctx, err := cmdtesting.RunCommand(c, cloud.NewRemoveCloudCommand(), "prodstack", "--client")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(cmdtesting.Stderr(ctx), tc.Equals, "Cannot remove public cloud \"prodstack\" from client\n")
}

func (s *removeSuite) TestCannotRemovePublicCloudWithCredentials(c *tc.C) {
	s.createTestCloudData(c)
	ctx, err := cmdtesting.RunCommand(c, cloud.NewRemoveCloudCommand(), "prodstack2", "--client")
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(cmdtesting.Stderr(ctx), tc.Equals, "Cannot remove public cloud \"prodstack2\" from client\n"+
		"To hide this cloud, remove it's credentials with `juju remove-credential`\n")
}

func (s *removeSuite) TestSpecifyingTargetControllerFlag(c *tc.C) {
	command := cloud.NewRemoveCloudCommandForTest(s.store, nil)
	_, err := cmdtesting.RunCommand(c, command, "fnord", "--target-controller=mycontroller-1")
	c.Assert(err, tc.ErrorIs, cmd.ErrCommandMissing)
}

func (s *removeSuite) TestCannotRemoveBuiltinCloud(c *tc.C) {
	defer s.setup(c).Finish()

	s.createTestCloudData(c)
	unregister := environs.RegisterProvider("test", s.testProvider)
	defer unregister()

	s.testProvider.MockCloudDetector.EXPECT().DetectClouds().Return([]jujucloud.Cloud{{Name: "foo-builtin"}}, nil)
	ctx, err := cmdtesting.RunCommand(c, cloud.NewRemoveCloudCommand(), "foo-builtin", "--client")

	c.Assert(err, tc.ErrorIsNil)
	c.Assert(cmdtesting.Stderr(ctx), tc.Equals, "Cannot remove built-in cloud \"foo-builtin\" from client\n")
}

func assertPersonalClouds(c *tc.C, names ...string) {
	personalClouds, err := jujucloud.PersonalCloudMetadata()
	c.Assert(err, tc.ErrorIsNil)
	actual := make([]string, 0, len(personalClouds))
	for name := range personalClouds {
		actual = append(actual, name)
	}
	c.Assert(actual, tc.SameContents, names)
}
