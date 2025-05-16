// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/domain/port/state"
)

var _ State = (*state.State)(nil)

//go:generate go run go.uber.org/mock/mockgen -typed -package service -destination package_mock_test.go github.com/juju/juju/domain/port/service State

func TestPackage(t *stdtesting.T) {
	tc.TestingT(t)
}
