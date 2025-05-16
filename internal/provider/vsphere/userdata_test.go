// Copyright 2015 Canonical Ltd.
// Copyright 2015 Cloudbase Solutions SRL
// Licensed under the AGPLv3, see LICENCE file for details.

package vsphere_test

import (
	"encoding/base64"
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/core/os/ostype"
	"github.com/juju/juju/internal/cloudconfig/cloudinit/cloudinittest"
	"github.com/juju/juju/internal/provider/vsphere"
	"github.com/juju/juju/internal/testing"
)

type UserdataSuite struct {
	testing.BaseSuite
}

func TestUserdataSuite(t *stdtesting.T) { tc.Run(t, &UserdataSuite{}) }
func (s *UserdataSuite) TestVsphereUnix(c *tc.C) {
	renderer := vsphere.VsphereRenderer{}
	cloudcfg := &cloudinittest.CloudConfig{YAML: []byte("yaml")}

	result, err := renderer.Render(cloudcfg, ostype.Ubuntu)
	c.Assert(err, tc.ErrorIsNil)
	expected := base64.StdEncoding.EncodeToString(cloudcfg.YAML)
	c.Assert(string(result), tc.DeepEquals, expected)
}

func (s *UserdataSuite) TestVsphereUnknownOS(c *tc.C) {
	renderer := vsphere.VsphereRenderer{}
	cloudcfg := &cloudinittest.CloudConfig{}
	result, err := renderer.Render(cloudcfg, ostype.GenericLinux)
	c.Assert(result, tc.IsNil)
	c.Assert(err, tc.ErrorMatches, "Cannot encode userdata for OS: GenericLinux")
}
