// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charmrevisioner

//go:generate go run go.uber.org/mock/mockgen -typed -package charmrevisioner -destination package_mocks_test.go github.com/juju/juju/internal/worker/charmrevisioner CharmhubClient,ModelConfigService,ApplicationService,ModelService,ResourceService
//go:generate go run go.uber.org/mock/mockgen -typed -package charmrevisioner -destination clock_mocks_test.go github.com/juju/clock Clock
//go:generate go run go.uber.org/mock/mockgen -typed -package charmrevisioner -destination http_mocks_test.go github.com/juju/juju/core/http HTTPClientGetter,HTTPClient

func ptr[T any](v T) *T {
	return &v
}
