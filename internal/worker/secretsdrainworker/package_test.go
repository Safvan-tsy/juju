// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package secretsdrainworker

//go:generate go run go.uber.org/mock/mockgen -typed -package mocks -destination mocks/secretsdrainworker_mock.go github.com/juju/juju/internal/worker/secretsdrainworker SecretsDrainFacade
//go:generate go run go.uber.org/mock/mockgen -typed -package mocks -destination mocks/secrets_mock.go github.com/juju/juju/internal/secrets BackendsClient
//go:generate go run go.uber.org/mock/mockgen -typed -package mocks -destination mocks/secretsprovider_mock.go github.com/juju/juju/internal/secrets/provider SecretsBackend
//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/leadership_mock.go github.com/juju/juju/core/leadership TrackerWorker
