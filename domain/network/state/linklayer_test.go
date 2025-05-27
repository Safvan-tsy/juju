// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"testing"

	"github.com/canonical/sqlair"
	"github.com/juju/tc"

	"github.com/juju/juju/core/machine"
	corenetwork "github.com/juju/juju/core/network"
	"github.com/juju/juju/domain/network"
	"github.com/juju/juju/domain/network/internal"
	schematesting "github.com/juju/juju/domain/schema/testing"
)

type linkLayerSuite struct {
	schematesting.ModelSuite
}

func TestLinkLayerSuite(t *testing.T) {
	tc.Run(t, &linkLayerSuite{})
}

func (s *linkLayerSuite) TestMachineInterfaceViewFitsType(c *tc.C) {
	db, err := s.TxnRunnerFactory()()
	c.Assert(err, tc.ErrorIsNil)

	nodeUUID := "net-node-uuid"
	machineUUID := "machine-uuid"
	machineName := "0"
	devUUID := "dev-uuid"
	devName := "eth0"
	subUUID := "sub-uuid"
	addrUUID := "addr-uuid"

	ctx := c.Context()

	err = db.StdTxn(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO net_node (uuid) VALUES (?)", nodeUUID); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, "INSERT INTO machine (uuid, net_node_uuid, name, life_id) VALUES (?, ?, ? ,?)",
			machineUUID, nodeUUID, machineName, 0,
		); err != nil {
			return err
		}

		insertLLD := `
INSERT INTO link_layer_device (uuid, net_node_uuid, name, mtu, mac_address, device_type_id, virtual_port_type_id) 
VALUES (?, ?, ?, ?, ?, ?, ?)`

		if _, err = tx.ExecContext(ctx, insertLLD, devUUID, nodeUUID, devName, 1500, "00:11:22:33:44:55", 0, 0); err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, "INSERT INTO subnet (uuid, cidr, space_uuid) VALUES (?, ?, ?)",
			subUUID, "10.0.0.0/24", corenetwork.AlphaSpaceId,
		); err != nil {
			return err
		}

		insertIPAddress := `
INSERT INTO ip_address (uuid, device_uuid, address_value, type_id, scope_id, origin_id, config_type_id, subnet_uuid, net_node_uuid) 
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

		_, err = tx.ExecContext(ctx, insertIPAddress, addrUUID, devUUID, "10.0.0.1", 0, 0, 0, 0, subUUID, nodeUUID)
		if err != nil {
			return err
		}

		return nil
	})
	c.Assert(err, tc.ErrorIsNil)

	stmt, err := sqlair.Prepare("SELECT &machineInterfaceRow.* FROM v_machine_interface", machineInterfaceRow{})
	c.Assert(err, tc.ErrorIsNil)

	var rows []machineInterfaceRow
	err = db.Txn(ctx, func(ctx context.Context, txn *sqlair.TX) error {
		return txn.Query(ctx, stmt).GetAll(&rows)
	})
	c.Assert(err, tc.ErrorIsNil)

	c.Assert(rows, tc.HasLen, 1)

	r := rows[0]
	c.Check(r.MachineUUID, tc.Equals, machineUUID)
	c.Check(r.MachineName, tc.Equals, machineName)
	c.Check(r.DeviceUUID, tc.Equals, devUUID)
	c.Check(r.DeviceName, tc.Equals, devName)
	c.Check(r.AddressUUID.String, tc.Equals, addrUUID)
	c.Check(r.SubnetUUID.String, tc.Equals, subUUID)
}

type linkLayerImportSuite struct {
	linkLayerBaseSuite
}

var _ = tc.Suite(&linkLayerImportSuite{})

func (s *linkLayerImportSuite) TestImportLinkLayerDevices(c *tc.C) {
	// Arrange:
	ctx := c.Context()

	// Arrange: prior imported items required for link layer devices.
	netNodeUUID := s.addNetNode(c)
	machineName := machine.Name("73")
	s.addMachine(c, machineName, netNodeUUID)

	// Arrange: data to be imported.
	importData := []internal.ImportLinkLayerDevice{
		{
			NetNodeUUID:      netNodeUUID,
			Name:             "test",
			MTU:              ptr(int64(1500)),
			Type:             network.DeviceTypeEthernet,
			VirtualPortType:  network.NonVirtualPortType,
			MachineID:        machineName,
			ParentDeviceName: "parent",
			ProviderID:       ptr(corenetwork.Id("one")),
			MACAddress:       ptr("00:16:3e:ad:4e:01"),
		},
		{
			NetNodeUUID:     netNodeUUID,
			Name:            "parent",
			MTU:             ptr(int64(1500)),
			Type:            network.DeviceTypeEthernet,
			VirtualPortType: network.NonVirtualPortType,
			MachineID:       machineName,
			ProviderID:      ptr(corenetwork.Id("two")),
			MACAddress:      ptr("00:16:3e:ad:4e:88"),
		},
	}

	// Act
	err := s.state.ImportLinkLayerDevices(ctx, importData)

	// Assert
	c.Check(err, tc.ErrorIsNil)
	s.checkRowCount(c, "link_layer_device", 2)
	s.checkRowCount(c, "link_layer_device_parent", 1)
	s.checkRowCount(c, "provider_link_layer_device", 2)
}

func (s *linkLayerImportSuite) TestDeleteImportedRelations(c *tc.C) {
	// Arrange:
	ctx := c.Context()

	// Arrange: prior imported items required for link layer devices.
	netNodeUUID := s.addNetNode(c)
	machineName := machine.Name("73")
	s.addMachine(c, machineName, netNodeUUID)

	// Arrange: import some data
	importData := []internal.ImportLinkLayerDevice{
		{
			NetNodeUUID:      netNodeUUID,
			Name:             "test",
			MTU:              ptr(int64(1500)),
			Type:             network.DeviceTypeEthernet,
			VirtualPortType:  network.NonVirtualPortType,
			MachineID:        machineName,
			ParentDeviceName: "parent",
			ProviderID:       ptr(corenetwork.Id("one")),
			MACAddress:       ptr("00:16:3e:ad:4e:01"),
		},
		{
			NetNodeUUID:     netNodeUUID,
			Name:            "parent",
			MTU:             ptr(int64(1500)),
			Type:            network.DeviceTypeEthernet,
			VirtualPortType: network.NonVirtualPortType,
			MachineID:       machineName,
			ProviderID:      ptr(corenetwork.Id("two")),
			MACAddress:      ptr("00:16:3e:ad:4e:88"),
		},
	}
	err := s.state.ImportLinkLayerDevices(ctx, importData)
	c.Assert(err, tc.ErrorIsNil)

	// Act
	err = s.state.DeleteImportedLinkLayerDevices(c.Context())

	// Assert
	c.Assert(err, tc.ErrorIsNil)
	s.checkRowCount(c, "link_layer_device", 0)
	s.checkRowCount(c, "link_layer_device_parent", 0)
	s.checkRowCount(c, "provider_link_layer_device", 0)
}
