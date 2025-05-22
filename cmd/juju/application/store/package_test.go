// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package store_test

import (
	stdtesting "testing"

	"github.com/juju/tc"
)

//go:generate go run go.uber.org/mock/mockgen -typed -package mocks -destination ./mocks/store_mock.go github.com/juju/juju/cmd/juju/application/store CharmAdder,CharmsAPI,DownloadBundleClient,CharmReader
//go:generate go run go.uber.org/mock/mockgen -typed -package mocks -destination ./mocks/charm_mock.go github.com/juju/juju/internal/charm Bundle

