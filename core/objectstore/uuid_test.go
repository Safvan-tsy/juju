// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package objectstore

import (
	stdtesting "testing"

	"github.com/juju/tc"

	coreerrors "github.com/juju/juju/core/errors"
	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/internal/uuid"
)

type ObjectStoreUUIDSuite struct {
	testhelpers.IsolationSuite
}

func TestObjectStoreUUIDSuite(t *stdtesting.T) {
	tc.Run(t, &ObjectStoreUUIDSuite{})
}

func (*ObjectStoreUUIDSuite) TestUUIDValidate(c *tc.C) {
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

func (*ObjectStoreUUIDSuite) TestUUIDIsEmpty(c *tc.C) {
	tests := []struct {
		uuid  string
		value bool
	}{
		{
			uuid:  "",
			value: true,
		},
		{
			uuid:  "invalid",
			value: false,
		},
		{
			uuid:  uuid.MustNewUUID().String(),
			value: false,
		},
	}

	for i, test := range tests {
		c.Logf("test %d: %q", i, test.uuid)
		empty := UUID(test.uuid).IsEmpty()

		c.Check(empty, tc.Equals, test.value)
	}
}
