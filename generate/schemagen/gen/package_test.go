// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package gen_test

import (
	stdtesting "testing"

	"github.com/juju/tc"
)

//go:generate go run go.uber.org/mock/mockgen -typed -package gen -destination describeapi_mock.go -write_package_comment=false github.com/juju/juju/generate/schemagen/gen APIServer,Registry,PackageRegistry

func TestPackage(t *stdtesting.T) {
	tc.TestingT(t)
}
