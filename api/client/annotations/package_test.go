// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package annotations

import (
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/api/base"
)

func TestAll(t *stdtesting.T) {
	tc.TestingT(t)
}

func NewClientFromCaller(caller base.FacadeCaller) *Client {
	return &Client{
		facade: caller,
	}
}
