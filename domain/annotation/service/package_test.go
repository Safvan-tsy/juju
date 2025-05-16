// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	stdtesting "testing"

	"github.com/juju/tc"
)

//go:generate go run go.uber.org/mock/mockgen -typed -package service -destination state_mock_test.go github.com/juju/juju/domain/annotation/service State

func TestPackage(t *stdtesting.T) {
	tc.TestingT(t)
}
