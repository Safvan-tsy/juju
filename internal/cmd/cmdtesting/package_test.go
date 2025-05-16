// Copyright 2017 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmdtesting_test

import (
	stdtesting "testing"

	"github.com/juju/tc"
)

func TestPackage(t *stdtesting.T) {
	tc.TestingT(t)
}
