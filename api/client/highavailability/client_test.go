// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package highavailability_test

import (
	"testing"

	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	basemocks "github.com/juju/juju/api/base/mocks"
	"github.com/juju/juju/api/client/highavailability"
	"github.com/juju/juju/rpc/params"
)

type clientSuite struct {
}

func TestClientSuite(t *testing.T) {
	tc.Run(t, &clientSuite{})
}

func (s *clientSuite) TestControllerDetails(c *tc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	res := new(params.ControllerDetailsResults)
	results := params.ControllerDetailsResults{
		Results: []params.ControllerDetails{{
			ControllerId: "666",
			APIAddresses: []string{"address"},
		}}}

	mockFacadeCaller := basemocks.NewMockFacadeCaller(ctrl)
	mockFacadeCaller.EXPECT().FacadeCall(gomock.Any(), "ControllerDetails", nil, res).SetArg(3, results).Return(nil)
	mockClient := basemocks.NewMockClientFacade(ctrl)
	mockClient.EXPECT().BestAPIVersion().Return(3)
	client := highavailability.NewClientFromCaller(mockFacadeCaller, mockClient)

	result, err := client.ControllerDetails(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(result, tc.DeepEquals, map[string]highavailability.ControllerDetails{
		"666": {
			ControllerID: "666",
			APIEndpoints: []string{"address"},
		},
	})
}
