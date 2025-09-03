// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package network

import (
	"context"
	"net"
	"strings"

	"github.com/juju/names/v6"

	"github.com/juju/juju/core/network"
	domainnetwork "github.com/juju/juju/domain/network"
	internallogger "github.com/juju/juju/internal/logger"
	"github.com/juju/juju/rpc/params"
)

var logger = internallogger.GetLogger("juju.apiserver.common.network")

func SubnetInfoToParamsSubnet(subnet network.SubnetInfo) params.Subnet {
	return params.Subnet{
		CIDR:              subnet.CIDR,
		VLANTag:           subnet.VLANTag,
		ProviderId:        subnet.ProviderId.String(),
		ProviderNetworkId: subnet.ProviderNetworkId.String(),
		Zones:             subnet.AvailabilityZones,
		SpaceTag:          names.NewSpaceTag(subnet.SpaceName.String()).String(),
		Life:              subnet.Life,
	}
}

// ParamsNetworkConfigToDomain transforms network config wire params to network
// interfaces recognised by the network domain.
func ParamsNetworkConfigToDomain(
	ctx context.Context, args []params.NetworkConfig, origin network.Origin,
) ([]domainnetwork.NetInterface, error) {
	nics := make([]domainnetwork.NetInterface, len(args))

	for i, arg := range args {
		nics[i] = domainnetwork.NetInterface{
			Name:             arg.InterfaceName,
			MTU:              nilIfEmpty(int64(arg.MTU)),
			MACAddress:       nilIfEmpty(arg.MACAddress),
			ProviderID:       nilIfEmpty(network.Id(arg.ProviderId)),
			Type:             network.LinkLayerDeviceType(arg.InterfaceType),
			VirtualPortType:  network.VirtualPortType(arg.VirtualPortType),
			IsAutoStart:      !arg.NoAutoStart,
			IsEnabled:        !arg.Disabled,
			ParentDeviceName: arg.ParentInterfaceName,
			GatewayAddress:   nilIfEmpty(arg.GatewayAddress),
			IsDefaultGateway: arg.IsDefaultGateway,
			VLANTag:          uint64(arg.VLANTag),
			DNSSearchDomains: arg.DNSSearchDomains,
			DNSAddresses:     arg.DNSServers,
		}

		var addrs []domainnetwork.NetAddr
		for _, addr := range arg.Addresses {
			addrs = append(addrs, domainnetwork.NetAddr{
				InterfaceName:    arg.InterfaceName,
				ProviderID:       nil,
				AddressValue:     ipWithCIDRMask(ctx, addr, arg.InterfaceName),
				ProviderSubnetID: nil,
				AddressType:      network.AddressType(addr.Type),
				ConfigType:       network.AddressConfigType(addr.ConfigType),
				Origin:           origin,
				Scope:            network.Scope(addr.Scope),
				IsSecondary:      addr.IsSecondary,
				IsShadow:         false,
			})
		}

		nics[i].Addrs = addrs
	}

	return nics, nil
}

func ipWithCIDRMask(ctx context.Context, addr params.Address, interfaceName string) string {
	// This handles *forward* compatibility at the time of writing,
	// where the address may already have a CIDR suffix.
	if strings.Contains(addr.Value, "/") {
		return addr.Value
	}

	ip := net.ParseIP(addr.Value)

	_, ipNet, _ := net.ParseCIDR(addr.CIDR)
	if ipNet != nil {
		ipNet.IP = ip
		return ipNet.String()
	}

	// This is not known to be possible at the time of writing.
	// We will still attempt to match the address to a known subnet ID.
	msg := "address %q for interface %q has no CIDR; using single IP suffix"
	logger.Warningf(ctx, msg, addr.Value, interfaceName)

	if ip.To4() != nil {
		return ip.String() + "/32"
	}
	return ip.String() + "/128"
}

func nilIfEmpty[T comparable](in T) *T {
	var empty T
	if in == empty {
		return nil
	}
	return &in
}
