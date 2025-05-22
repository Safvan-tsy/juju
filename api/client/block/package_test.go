// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package block

import (
	"github.com/juju/juju/api/base"
)

func NewClientFromCaller(caller base.FacadeCaller) *Client {
	return &Client{
		facade: caller,
	}
}
