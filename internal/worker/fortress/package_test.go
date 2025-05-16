// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package fortress_test

import (
	stdtesting "testing"

	"github.com/juju/tc"
	"go.uber.org/goleak"
)

func TestPackage(t *stdtesting.T) {
	defer goleak.VerifyNone(t)

	tc.TestingT(t)
}
