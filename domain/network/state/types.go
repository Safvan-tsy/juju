// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"database/sql"

	"github.com/juju/juju/core/machine"
	corenetwork "github.com/juju/juju/core/network"
	"github.com/juju/juju/domain/network"
	"github.com/juju/juju/internal/errors"
)

type entityUUID struct {
	// UUID uniquely identifies an entity.
	UUID string `db:"uuid"`
}

// subnet represents a single row from the subnet table.
type subnet struct {
	// UUID is the subnet's UUID.
	UUID string `db:"uuid"`
	// CIDR of the network, in 123.45.67.89/24 format.
	CIDR string `db:"cidr"`
	// VLANtag is the subnet's vlan tag.
	VLANtag int `db:"vlan_tag"`
	// SpaceUUID is the space UUID.
	SpaceUUID string `db:"space_uuid"`
}

// providerSubnet represents a single row from the provider_subnet table.
type providerSubnet struct {
	// SubnetUUID is the UUID of the subnet.
	SubnetUUID string `db:"subnet_uuid"`
	// ProviderID is the provider-specific subnet ID.
	ProviderID corenetwork.Id `db:"provider_id"`
}

// providerNetwork represents a single row from the provider_network table.
type providerNetwork struct {
	// ProviderNetworkUUID is the provider network UUID.
	ProviderNetworkUUID string `db:"uuid"`
	// ProviderNetworkID is the provider-specific ID of the network
	// containing this subnet.
	ProviderNetworkID corenetwork.Id `db:"provider_network_id"`
}

// providerNetworkSubnet represents a single row from the provider_network_subnet mapping table.
type providerNetworkSubnet struct {
	// SubnetUUID is the UUID of the subnet.
	SubnetUUID string `db:"subnet_uuid"`
	// ProviderNetworkUUID is the provider network UUID.
	ProviderNetworkUUID string `db:"provider_network_uuid"`
}

// space represents a single row from the space table.
type space struct {
	// Name is the space name.
	Name string `db:"name"`
	// UUID is the unique ID of the space.
	UUID string `db:"uuid"`
}

type spaceName struct {
	Name string `db:"name"`
}

type countResult struct {
	Count int `db:"count"`
}

// providerSpace represents a single row from the provider_space table.
type providerSpace struct {
	// SpaceUUID is the unique ID of the space.
	SpaceUUID string `db:"space_uuid"`
	// ProviderID is a provider-specific space ID.
	ProviderID corenetwork.Id `db:"provider_id"`
}

// availabilityZone represents a row from the availability_zone table.
type availabilityZone struct {
	// Name is the name of the availability zone.
	Name string `db:"name"`
	// UUID is the unique ID of the availability zone.
	UUID string `db:"uuid"`
}

// availabilityZoneSubnet represents a row from the availability_zone_subnet
// table.
type availabilityZoneSubnet struct {
	// UUID is the unique ID of the availability zone.
	AZUUID string `db:"availability_zone_uuid"`
	// SubnetUUID is the unique ID of the Subnet.
	SubnetUUID string `db:"subnet_uuid"`
}

// SubnetRow represents the subnet fields of a single row from the
// v_space_subnets view.
type SubnetRow struct {
	// UUID is the subnet UUID.
	UUID string `db:"subnet_uuid"`

	// CIDR is the one of the subnet's cidr.
	CIDR string `db:"subnet_cidr"`

	// VLANTag is the subnet's vlan tag.
	VLANTag int `db:"subnet_vlan_tag"`

	// ProviderID is the subnet's provider id.
	ProviderID string `db:"subnet_provider_id"`

	// ProviderNetworkID is the subnet's provider network id.
	ProviderNetworkID string `db:"subnet_provider_network_id"`

	// ProviderSpaceUUID is the subnet's space uuid.
	ProviderSpaceUUID sql.NullString `db:"subnet_provider_space_uuid"`

	// SpaceUUID is the space uuid.
	SpaceUUID sql.NullString `db:"subnet_space_uuid"`

	// SpaceName is the name of the space the subnet is associated with.
	SpaceName sql.NullString `db:"subnet_space_name"`

	// AZ is the availability zones on the subnet.
	AZ string `db:"subnet_az"`
}

// SpaceSubnetRow represents a single row from the v_space_subnets view.
type spaceSubnetRow struct {
	// SubnetRow is embedded by SpaceSubnetRow since every row corresponds to a
	// subnet of the space. This allows SubnetRow to be
	SubnetRow

	// UUID is the space UUID.
	SpaceUUID string `db:"uuid"`

	// Name is the space name.
	SpaceName string `db:"name"`

	// ProviderID is the space provider id.
	SpaceProviderID sql.NullString `db:"provider_id"`
}

// SpaceSubnetRows is a slice of SpaceSubnet rows.
type SpaceSubnetRows []spaceSubnetRow

// subnetRows is a slice of Subnet rows.
type subnetRows []SubnetRow

// ToSpaceInfos converts Spaces to a slice of network.SpaceInfo structs.
// This method makes sure only unique subnets are mapped and flattens them into
// each space.
// No sorting is applied.
func (sp SpaceSubnetRows) ToSpaceInfos() corenetwork.SpaceInfos {
	var res corenetwork.SpaceInfos

	// Prepare structs for unique subnets for each space.
	uniqueAZs := make(map[string]map[string]map[string]string)
	uniqueSubnets := make(map[string]map[string]corenetwork.SubnetInfo)
	uniqueSpaces := make(map[string]corenetwork.SpaceInfo)

	for _, spaceSubnet := range sp {
		spInfo := corenetwork.SpaceInfo{
			ID:   spaceSubnet.SpaceUUID,
			Name: corenetwork.SpaceName(spaceSubnet.SpaceName),
		}

		if spaceSubnet.SpaceProviderID.Valid {
			spInfo.ProviderId = corenetwork.Id(spaceSubnet.SpaceProviderID.String)
		}
		uniqueSpaces[spaceSubnet.SpaceUUID] = spInfo

		snInfo := spaceSubnet.SubnetRow.ToSubnetInfo()
		if snInfo != nil {
			if _, ok := uniqueSubnets[spaceSubnet.SpaceUUID]; !ok {
				uniqueSubnets[spaceSubnet.SpaceUUID] = make(map[string]corenetwork.SubnetInfo)
			}

			uniqueSubnets[spaceSubnet.SpaceUUID][spaceSubnet.UUID] = *snInfo

			if _, ok := uniqueAZs[spaceSubnet.SpaceUUID]; !ok {
				uniqueAZs[spaceSubnet.SpaceUUID] = make(map[string]map[string]string)
			}
			if _, ok := uniqueAZs[spaceSubnet.SpaceUUID][spaceSubnet.UUID]; !ok {
				uniqueAZs[spaceSubnet.SpaceUUID][spaceSubnet.UUID] = make(map[string]string)
			}
			uniqueAZs[spaceSubnet.SpaceUUID][spaceSubnet.UUID][spaceSubnet.AZ] = spaceSubnet.AZ
		}
	}

	// Iterate through every space and flatten its subnets.
	for spaceUUID, space := range uniqueSpaces {
		space.Subnets = flattenAZs(uniqueSubnets[spaceUUID], uniqueAZs[spaceUUID])
		res = append(res, space)
	}

	return res
}

// ToSubnetInfo deserializes a row containing subnet fields to a SubnetInfo
// struct.
func (s SubnetRow) ToSubnetInfo() *corenetwork.SubnetInfo {
	// Make sure we don't add empty rows as empty subnets.
	if s.UUID == "" {
		return nil
	}
	sInfo := corenetwork.SubnetInfo{
		ID:                corenetwork.Id(s.UUID),
		CIDR:              s.CIDR,
		VLANTag:           s.VLANTag,
		ProviderId:        corenetwork.Id(s.ProviderID),
		ProviderNetworkId: corenetwork.Id(s.ProviderNetworkID),
	}
	if s.ProviderSpaceUUID.Valid {
		sInfo.ProviderSpaceId = corenetwork.Id(s.ProviderSpaceUUID.String)
	}
	if s.SpaceUUID.Valid {
		sInfo.SpaceID = s.SpaceUUID.String
	}
	if s.SpaceName.Valid {
		sInfo.SpaceName = s.SpaceName.String
	}

	return &sInfo
}

// ToSubnetInfos converts Subnets to a slice of network.SubnetInfo structs.
// This method makes sure only unique AZs are mapped and flattens them into
// each subnet.
// No sorting is applied.
func (sn subnetRows) ToSubnetInfos() corenetwork.SubnetInfos {
	// Prepare structs for unique subnets.
	uniqueAZs := make(map[string]map[string]string)
	uniqueSubnets := make(map[string]corenetwork.SubnetInfo)

	for _, subnet := range sn {
		subnetInfo := subnet.ToSubnetInfo()
		if subnetInfo != nil {
			uniqueSubnets[subnet.UUID] = *subnetInfo

			if _, ok := uniqueAZs[subnet.UUID]; !ok {
				uniqueAZs[subnet.UUID] = make(map[string]string)
			}
			uniqueAZs[subnet.UUID][subnet.AZ] = subnet.AZ
		}
	}

	return flattenAZs(uniqueSubnets, uniqueAZs)
}

// flattenAZs iterates over every subnet and flattens its AZs.
func flattenAZs(
	uniqueSubnets map[string]corenetwork.SubnetInfo,
	uniqueAZs map[string]map[string]string,
) corenetwork.SubnetInfos {
	var subnets corenetwork.SubnetInfos

	for subnetUUID, subnet := range uniqueSubnets {
		var availabilityZones []string
		for _, availabilityZone := range uniqueAZs[subnetUUID] {
			availabilityZones = append(availabilityZones, availabilityZone)
		}
		subnet.AvailabilityZones = availabilityZones

		subnets = append(subnets, subnet)
	}

	return subnets
}

// linkLayerDeviceDML is for writing data to the link_layer_device table.
type linkLayerDeviceDML struct {
	UUID              string  `db:"uuid"`
	NetNodeUUID       string  `db:"net_node_uuid"`
	Name              string  `db:"name"`
	MTU               *int64  `db:"mtu"`
	MACAddress        *string `db:"mac_address"`
	DeviceTypeID      int64   `db:"device_type_id"`
	VirtualPortTypeID int64   `db:"virtual_port_type_id"`
	IsAutoStart       bool    `db:"is_auto_start"`
	IsEnabled         bool    `db:"is_enabled"`
	IsDefaultGateway  bool    `db:"is_default_gateway"`
	GatewayAddress    *string `db:"gateway_address"`
	VlanTag           uint64  `db:"vlan_tag"`
}

// netInterfaceToDML returns persistence types for representing a single
// [network.NetInterface] instance without its addresses.
// The incoming map of device name to device UUID should contain entries for
// this device's UUID and its parent device if required.
// It is expected that the map will be populated as part of the reconciliation
// process prior to calling this method.
func netInterfaceToDML(
	dev network.NetInterface, nodeUUID string, nameToUUID map[string]string,
) (linkLayerDeviceDML, error) {
	var dml linkLayerDeviceDML

	devUUID, ok := nameToUUID[dev.Name]
	if !ok {
		return dml, errors.Errorf("no UUID associated with device %q", dev.Name)
	}

	devTypeID, err := encodeDeviceType(dev.Type)
	if err != nil {
		return dml, errors.Capture(err)
	}

	portTypeID, err := encodeVirtualPortType(dev.VirtualPortType)
	if err != nil {
		return dml, errors.Capture(err)
	}

	dml = linkLayerDeviceDML{
		UUID:              devUUID,
		NetNodeUUID:       nodeUUID,
		Name:              dev.Name,
		MTU:               dev.MTU,
		MACAddress:        dev.MACAddress,
		DeviceTypeID:      devTypeID,
		VirtualPortTypeID: portTypeID,
		IsAutoStart:       dev.IsAutoStart,
		IsEnabled:         dev.IsEnabled,
		IsDefaultGateway:  dev.IsDefaultGateway,
		GatewayAddress:    dev.GatewayAddress,
		VlanTag:           dev.VLANTag,
	}

	// TODO (manadart 2025-05-02): This needs to return additional types for:
	// - link_layer_device_parent
	// - provider_link_layer_device
	// - link_layer_device_dns_domain
	// - link_layer_device_dns_address

	return dml, nil
}

// encodeDeviceType returns an identifier congruent with the database lookup for
// a network interface type. The caller of this method should already have
// called IsValidLinkLayerDeviceType for the input in the service layer,
// but we guard against bad input anyway.
func encodeDeviceType(kind corenetwork.LinkLayerDeviceType) (int64, error) {
	switch kind {
	case corenetwork.UnknownDevice:
		return 0, nil
	case corenetwork.LoopbackDevice:
		return 1, nil
	case corenetwork.EthernetDevice:
		return 2, nil
	case corenetwork.VLAN8021QDevice:
		return 3, nil
	case corenetwork.BondDevice:
		return 4, nil
	case corenetwork.BridgeDevice:
		return 5, nil
	case corenetwork.VXLANDevice:
		return 6, nil
	default:
		return -1, errors.Errorf("unsupported device type: %q", kind)
	}
}

// encodeVirtualPortType returns an identifier congruent with the database
// lookup for a virtual port type. The caller of this method should have already
// validated the input in the service layer.
func encodeVirtualPortType(kind corenetwork.VirtualPortType) (int64, error) {
	switch kind {
	case corenetwork.NonVirtualPort:
		return 0, nil
	case corenetwork.OvsPort:
		return 1, nil
	default:
		return -1, errors.Errorf("unsupported virtual port type: %q", kind)
	}
}

// ipAddressDML is for writing data to the ip_address table.
type ipAddressDML struct {
	UUID         string  `db:"uuid"`
	NodeUUID     string  `db:"net_node_uuid"`
	DeviceUUID   string  `db:"device_uuid"`
	AddressValue string  `db:"address_value"`
	SubnetUUID   *string `db:"subnet_uuid"`
	TypeID       int64   `db:"type_id"`
	ConfigTypeID int64   `db:"config_type_id"`
	OriginID     int64   `db:"origin_id"`
	ScopeID      int64   `db:"scope_id"`
	IsSecondary  bool    `db:"is_secondary"`
	IsShadow     bool    `db:"is_shadow"`
}

// netAddrToDML returns a persistence type for representing a single
// [network.NetAddr].
// The incoming map of device name to device UUID should contain an entry for
// the device to which the address is assigned.
// The incoming map of IP address to UUID should contain an entry for this
// address.
// It is expected that the maps will be populated as part of the reconciliation
// process prior to calling this method.
func netAddrToDML(
	addr network.NetAddr, nodeUUID, devUUID string, ipToUUID map[string]string,
) (ipAddressDML, error) {
	var dml ipAddressDML

	addrUUID, ok := ipToUUID[addr.AddressValue]
	if !ok {
		return dml, errors.Errorf("no UUID associated with IP %q on device %q", addr.AddressValue, addr.InterfaceName)
	}

	addrTypeID, err := encodeAddressType(addr.AddressType)
	if err != nil {
		return dml, errors.Capture(err)
	}

	addrConfTypeID, err := encodeAddressConfigType(addr.ConfigType)
	if err != nil {
		return dml, errors.Capture(err)
	}

	originID, err := encodeAddressOrigin(addr.Origin)
	if err != nil {
		return dml, errors.Capture(err)
	}

	scopeID, err := encodeAddressScope(addr.Scope)
	if err != nil {
		return dml, errors.Capture(err)
	}

	dml = ipAddressDML{
		UUID:         addrUUID,
		NodeUUID:     nodeUUID,
		DeviceUUID:   devUUID,
		AddressValue: addr.AddressValue,
		SubnetUUID:   nil,
		TypeID:       addrTypeID,
		ConfigTypeID: addrConfTypeID,
		OriginID:     originID,
		ScopeID:      scopeID,
		IsSecondary:  addr.IsSecondary,
		IsShadow:     addr.IsShadow,
	}

	return dml, nil
}

func encodeAddressType(kind corenetwork.AddressType) (int64, error) {
	switch kind {
	case corenetwork.IPv4Address:
		return 0, nil
	case corenetwork.IPv6Address:
		return 1, nil
	case corenetwork.HostName:
		return -1, errors.Errorf("address type %q can not be used for an IP address", kind)
	default:
		return -1, errors.Errorf("unsupported address type: %q", kind)
	}
}

func encodeAddressConfigType(kind corenetwork.AddressConfigType) (int64, error) {
	switch kind {
	case corenetwork.ConfigUnknown:
		return 0, nil
	case corenetwork.ConfigDHCP:
		return 1, nil
	case corenetwork.ConfigStatic:
		return 4, nil
	case corenetwork.ConfigManual:
		return 5, nil
	case corenetwork.ConfigLoopback:
		return 6, nil
	default:
		return -1, errors.Errorf("unsupported address config type: %q", kind)
	}
}

func encodeAddressOrigin(kind corenetwork.Origin) (int64, error) {
	switch kind {
	case corenetwork.OriginMachine:
		return 0, nil
	case corenetwork.OriginProvider:
		return 1, nil
	default:
		return -1, errors.Errorf("unsupported address origin: %q", kind)
	}
}

func encodeAddressScope(kind corenetwork.Scope) (int64, error) {
	switch kind {
	case corenetwork.ScopeUnknown:
		return 0, nil
	case corenetwork.ScopePublic:
		return 1, nil
	case corenetwork.ScopeCloudLocal:
		return 4, nil
	case corenetwork.ScopeMachineLocal:
		return 5, nil
	case corenetwork.ScopeLinkLocal:
		return 6, nil
	default:
		return -1, errors.Errorf("unsupported address scope: %q", kind)
	}
}

// machineInterfaceRow is the type for a row from the v_machine_interface view.
type machineInterfaceRow struct {
	// MachineUUID and associated machine fields.
	MachineUUID string `db:"machine_uuid"`
	MachineName string `db:"machine_name"`
	NetNodeUUID string `db:"net_node_uuid"`

	// DeviceUUID and associated link-layer device fields.
	DeviceUUID        string         `db:"device_uuid"`
	DeviceName        string         `db:"device_name"`
	MTU               sql.NullInt64  `db:"mtu"`
	MacAddress        sql.NullString `db:"mac_address"`
	ProviderID        sql.NullString `db:"device_provider_id"`
	DeviceTypeID      int64          `db:"device_type_id"`
	VirtualPortTypeID int64          `db:"virtual_port_type_id"`
	IsAutoStart       bool           `db:"is_auto_start"`
	IsEnabled         bool           `db:"is_enabled"`
	ParentDeviceUUID  sql.NullString `db:"parent_device_uuid"`
	ParentDeviceName  sql.NullString `db:"parent_device_name"`
	GatewayAddress    sql.NullString `db:"gateway_address"`
	IsDefaultGateway  bool           `db:"is_default_gateway"`
	VLANTag           uint64         `db:"vlan_tag"`
	DNSAddress        sql.NullString `db:"dns_address"`
	DNSSearchDomain   sql.NullString `db:"search_domain"`

	// AddressUUID and associated IP address fields.
	AddressUUID       sql.NullString `db:"address_uuid"`
	ProviderAddressID sql.NullString `db:"provider_address_id"`
	AddressValue      sql.NullString `db:"address_value"`
	SubnetUUID        sql.NullString `db:"subnet_uuid"`
	CIDR              sql.NullString `db:"cidr"`
	ProviderSubnetID  sql.NullString `db:"provider_subnet_id"`
	AddressTypeID     sql.NullInt64  `db:"address_type_id"`
	ConfigTypeID      sql.NullInt64  `db:"config_type_id"`
	OriginID          sql.NullInt64  `db:"origin_id"`
	ScopeID           sql.NullInt64  `db:"scope_id"`
	IsSecondary       sql.NullBool   `db:"is_secondary"`
	IsShadow          sql.NullBool   `db:"is_shadow"`
}

type machineNameNetNode struct {
	MachineName machine.Name            `db:"name"`
	NetNodeUUID corenetwork.NetNodeUUID `db:"net_node_uuid"`
}

// linkLayerDevice is slightly different from linkLayerDeviceDML
// It's used to import LLDs .
type linkLayerDevice struct {
	UUID        corenetwork.LinkLayerDeviceUUID `db:"uuid"`
	NetNodeUUID corenetwork.NetNodeUUID         `db:"net_node_uuid"`
	Name        string                          `db:"name"`
	MTU         sql.NullInt64                   `db:"mtu"`
	MAC         sql.NullString                  `db:"mac_address"`
	// GatewayAddress is not provided in the first round of
	// model migration data from the link layer devices.
	// By using sql.NullString, we ensure the value is NULL
	// until it's available.
	GatewayAddress  sql.NullString          `db:"gateway_address"`
	IsAutoStart     bool                    `db:"is_auto_start"`
	IsEnabled       bool                    `db:"is_enabled"`
	Type            network.DeviceType      `db:"device_type_id"`
	VirtualPortType network.VirtualPortType `db:"virtual_port_type_id"`
	VLAN            int                     `db:"vlan_tag"`
}

type linkLayerDeviceParent struct {
	DeviceUUID corenetwork.LinkLayerDeviceUUID `db:"device_uuid"`
	ParentUUID corenetwork.LinkLayerDeviceUUID `db:"parent_uuid"`
}

type providerLinkLayerDevice struct {
	ProviderID corenetwork.Id                  `db:"provider_id"`
	DeviceUUID corenetwork.LinkLayerDeviceUUID `db:"device_uuid"`
}
