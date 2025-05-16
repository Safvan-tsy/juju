// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package uniter_test

import (
	stdtesting "testing"

	"github.com/juju/names/v6"
	"github.com/juju/tc"

	"github.com/juju/juju/api/agent/uniter"
	"github.com/juju/juju/api/base/testing"
	coretesting "github.com/juju/juju/internal/testing"
	"github.com/juju/juju/rpc/params"
)

type cloudNativeUniterSuite struct {
	coretesting.BaseSuite
}

func TestCloudNativeUniterSuite(t *stdtesting.T) { tc.Run(t, &cloudNativeUniterSuite{}) }
func (s *cloudNativeUniterSuite) TestCloudSpec(c *tc.C) {
	apiCaller := testing.APICallerFunc(func(objType string, version int, id, request string, arg, result interface{}) error {
		c.Assert(objType, tc.Equals, "Uniter")
		c.Assert(request, tc.Equals, "CloudSpec")
		c.Assert(arg, tc.IsNil)
		c.Assert(result, tc.FitsTypeOf, &params.CloudSpecResult{})
		*(result.(*params.CloudSpecResult)) = params.CloudSpecResult{
			Result: &params.CloudSpec{
				Name: "dummy",
				Credential: &params.CloudCredential{
					Attributes: map[string]string{
						"username": "dummy",
						"password": "secret",
					},
				},
			},
		}
		return nil
	})
	client := uniter.NewClient(apiCaller, names.NewUnitTag("wordpress/0"))

	result, err := client.CloudSpec(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(result.Name, tc.Equals, "dummy")
	c.Assert(result.Credential.Attributes, tc.DeepEquals, map[string]string{
		"username": "dummy",
		"password": "secret",
	})
}
