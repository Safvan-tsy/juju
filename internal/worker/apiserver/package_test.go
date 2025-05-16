// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package apiserver_test

import (
	stdtesting "testing"

	mgotesting "github.com/juju/mgo/v3/testing"
)

//go:generate go run go.uber.org/mock/mockgen -typed -package apiserver_test -destination controllerconfig_mock_test.go github.com/juju/juju/internal/worker/apiserver ControllerConfigService,ModelService
//go:generate go run go.uber.org/mock/mockgen -typed -package apiserver_test -destination service_mock_test.go github.com/juju/juju/internal/services DomainServicesGetter

func TestPackage(t *stdtesting.T) {
	mgotesting.MgoServer.EnableReplicaSet = true
	mgotesting.MgoTestPackage(t, nil)
}
