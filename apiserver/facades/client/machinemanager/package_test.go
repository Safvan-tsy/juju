// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package machinemanager

//go:generate go run go.uber.org/mock/mockgen -typed -package machinemanager -destination package_mock_test.go github.com/juju/juju/apiserver/facades/client/machinemanager Authorizer,StorageInterface,CharmhubClient,ControllerConfigService,MachineService,ApplicationService,NetworkService,KeyUpdaterService,ModelConfigService,BlockCommandService,AgentBinaryService,AgentPasswordService,ControllerNodeService,StatusService,RemovalService
//go:generate go run go.uber.org/mock/mockgen -typed -package machinemanager -destination state_mock_test.go github.com/juju/juju/state StorageAttachment,StorageInstance
//go:generate go run go.uber.org/mock/mockgen -typed -package machinemanager -destination volume_access_mock_test.go github.com/juju/juju/apiserver/common/storagecommon VolumeAccess
//go:generate go run go.uber.org/mock/mockgen -typed -package machinemanager -destination environ_mock_test.go github.com/juju/juju/environs Environ,InstanceTypesFetcher,BootstrapEnviron
//go:generate go run go.uber.org/mock/mockgen -typed -package machinemanager -destination objectstore_mock_test.go github.com/juju/juju/core/objectstore ObjectStore
