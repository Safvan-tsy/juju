// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package resources_test

//go:generate go run go.uber.org/mock/mockgen -typed -package mocks -destination ../mocks/resource_mock.go github.com/juju/juju/internal/worker/uniter/runner/context/resources OpenedResourceClient
