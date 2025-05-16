// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package keymanager

import (
	stdtesting "testing"

	"github.com/juju/tc"
)

//go:generate go run go.uber.org/mock/mockgen -typed -package keymanager -destination keymanager_mock.go github.com/juju/juju/apiserver/facades/client/keymanager BlockChecker
//go:generate go run go.uber.org/mock/mockgen -typed -package keymanager -destination service_mock.go github.com/juju/juju/apiserver/facades/client/keymanager KeyManagerService,UserService

func TestAll(t *stdtesting.T) {
	tc.TestingT(t)
}
