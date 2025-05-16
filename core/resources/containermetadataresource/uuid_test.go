// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package containermetadataresource

import (
	stdtesting "testing"

	"github.com/juju/tc"

	coreerrors "github.com/juju/juju/core/errors"
	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/internal/uuid"
)

type ContainerMetadataResourceUUIDSuite struct {
	testhelpers.IsolationSuite
}

func TestContainerMetadataResourceUUIDSuite(t *stdtesting.T) {
	tc.Run(t, &ContainerMetadataResourceUUIDSuite{})
}
func (*ContainerMetadataResourceUUIDSuite) TestIDValidate(c *tc.C) {
	tests := []struct {
		uuid string
		err  error
	}{
		{
			uuid: "",
			err:  coreerrors.NotValid,
		},
		{
			uuid: "invalid",
			err:  coreerrors.NotValid,
		},
		{
			uuid: uuid.MustNewUUID().String(),
		},
	}

	for i, test := range tests {
		c.Logf("test %d: %q", i, test.uuid)
		err := UUID(test.uuid).Validate()

		if test.err == nil {
			c.Check(err, tc.IsNil)
			continue
		}

		c.Check(err, tc.ErrorIs, test.err)
	}
}
