// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"testing"

	"github.com/juju/tc"

	domainlife "github.com/juju/juju/domain/life"
	domainnetwork "github.com/juju/juju/domain/network"
	"github.com/juju/juju/domain/storageprovisioning"
	"github.com/juju/juju/internal/uuid"
)

// volumeSuite provides a set of tests for asserting the state interface for
// volumes in the model.
type volumeSuite struct {
	baseSuite
}

// TestVolumeSuite runs the tests defined in [volumeSuite].
func TestVolumeSuite(t *testing.T) {
	tc.Run(t, &volumeSuite{})
}

// TestGetVolumeAttachmentIDsOnlyUnits tests that when requesting ids for a
// volume attachment and no machines are using the net node the unit name is
// reported.
func (s *volumeSuite) TestGetVolumeAttachmentIDsOnlyUnits(c *tc.C) {
	netNodeUUID := s.newNetNode(c)
	appUUID := s.newApplication(c, "foo")
	_, unitName := s.newUnitWithNetNode(c, "foo/0", appUUID, netNodeUUID)

	vsUUID, vsID := s.newMachineVolume(c)
	vsaUUID := s.newMachineVolumeAttachment(c, vsUUID, netNodeUUID)

	st := NewState(s.TxnRunnerFactory())
	result, err := st.GetVolumeAttachmentIDs(c.Context(), []string{vsaUUID})
	c.Check(err, tc.ErrorIsNil)
	c.Check(result, tc.DeepEquals, map[string]storageprovisioning.VolumeAttachmentID{
		vsaUUID: {
			VolumeID:    vsID,
			MachineName: nil,
			UnitName:    &unitName,
		},
	})
}

// TestGetVolumeAttachmentIDsOnlyMachines tests that when requesting ids for a
// volume attachment and the net node is attached to a machine the machine
// name is set.
func (s *volumeSuite) TestGetVolumeAttachmentIDsOnlyMachines(c *tc.C) {
	netNodeUUID := s.newNetNode(c)
	_, machineName := s.newMachineWithNetNode(c, netNodeUUID)

	vsUUID, vsID := s.newMachineVolume(c)
	vsaUUID := s.newMachineVolumeAttachment(c, vsUUID, netNodeUUID)

	st := NewState(s.TxnRunnerFactory())
	result, err := st.GetVolumeAttachmentIDs(c.Context(), []string{vsaUUID})
	c.Check(err, tc.ErrorIsNil)
	c.Check(result, tc.DeepEquals, map[string]storageprovisioning.VolumeAttachmentID{
		vsaUUID: {
			VolumeID:    vsID,
			MachineName: &machineName,
			UnitName:    nil,
		},
	})
}

// TestGetVolumeAttachmentIDsMachineNotUnit tests that when requesting ids for a
// volume attachment and the net node is attached to a machine the machine
// name is set. This should remain true when the net node is also used by a
// unit. This is a valid case when units are assigned to a machine.
func (s *volumeSuite) TestGetVolumeAttachmentIDsMachineNotUnit(c *tc.C) {
	netNodeUUID := s.newNetNode(c)
	_, machineName := s.newMachineWithNetNode(c, netNodeUUID)
	appUUID := s.newApplication(c, "foo")
	s.newUnitWithNetNode(c, "foo/0", appUUID, netNodeUUID)

	vsUUID, vsID := s.newMachineVolume(c)
	vsaUUID := s.newMachineVolumeAttachment(c, vsUUID, netNodeUUID)

	st := NewState(s.TxnRunnerFactory())
	result, err := st.GetVolumeAttachmentIDs(c.Context(), []string{vsaUUID})
	c.Check(err, tc.ErrorIsNil)
	c.Check(result, tc.DeepEquals, map[string]storageprovisioning.VolumeAttachmentID{
		vsaUUID: {
			VolumeID:    vsID,
			MachineName: &machineName,
			UnitName:    nil,
		},
	})
}

// TestGetVolumeAttachmentIDsMixed tests that when requesting ids for a
// mixed set of volume attachments uuids the machine name and unit name are
// correctly set.
func (s *volumeSuite) TestGetVolumeAttachmentIDsMixed(c *tc.C) {
	netNodeUUID1 := s.newNetNode(c)
	netNodeUUID2 := s.newNetNode(c)
	_, machineName := s.newMachineWithNetNode(c, netNodeUUID1)
	appUUID := s.newApplication(c, "foo")
	_, unitName := s.newUnitWithNetNode(c, "foo/0", appUUID, netNodeUUID2)

	vsOneUUID, vsOneID := s.newMachineVolume(c)
	vsaOneUUID := s.newMachineVolumeAttachment(c, vsOneUUID, netNodeUUID1)

	vsTwoUUID, vsTwoID := s.newMachineVolume(c)
	vsaTwoUUID := s.newMachineVolumeAttachment(c, vsTwoUUID, netNodeUUID2)

	st := NewState(s.TxnRunnerFactory())
	result, err := st.GetVolumeAttachmentIDs(c.Context(), []string{
		vsaOneUUID, vsaTwoUUID,
	})
	c.Check(err, tc.ErrorIsNil)
	c.Check(result, tc.DeepEquals, map[string]storageprovisioning.VolumeAttachmentID{
		vsaOneUUID: {
			VolumeID:    vsOneID,
			MachineName: &machineName,
			UnitName:    nil,
		},
		vsaTwoUUID: {
			VolumeID:    vsTwoID,
			MachineName: nil,
			UnitName:    &unitName,
		},
	})
}

// TestGetVolumeAttachmentIDsNotMachineOrUnit tests that when requesting
// ids for a volume attachment that is using a net node not attached to a
// machine or unit the uuid is dropped from the final result.
func (s *volumeSuite) TestGetVolumeAttachmentIDsNotMachineOrUnit(c *tc.C) {
	netNodeUUID := s.newNetNode(c)
	vsUUID, _ := s.newMachineVolume(c)
	vsaUUID := s.newMachineVolumeAttachment(c, vsUUID, netNodeUUID)

	st := NewState(s.TxnRunnerFactory())
	result, err := st.GetVolumeAttachmentIDs(c.Context(), []string{vsaUUID})
	c.Check(err, tc.ErrorIsNil)
	c.Check(result, tc.HasLen, 0)
}

// TestGetVolumeAttachmentIDsNotFound tests that when requesting ids for
// volume attachment uuids that don't exist the uuids are excluded from the
// result with no error returned.
func (s *volumeSuite) TestGetVolumeAttachmentIDsNotFound(c *tc.C) {
	st := NewState(s.TxnRunnerFactory())
	result, err := st.GetVolumeAttachmentIDs(c.Context(), []string{"no-exist"})
	c.Check(err, tc.ErrorIsNil)
	c.Check(result, tc.HasLen, 0)
}

// TestGetVolumeAttachmentLifeForNetNode tests that the correct life is
// reported for each model provisioned volume attachment associated with the
// given net node.
//
// We also inject a life change during the test to make sure that it is
// reflected.
func (s *volumeSuite) TestGetVolumeAttachmentLifeForNetNode(c *tc.C) {
	netNodeUUID := s.newNetNode(c)
	vsUUID1, _ := s.newMachineVolume(c)
	vsUUID2, _ := s.newMachineVolume(c)
	vsUUID3, _ := s.newMachineVolume(c)
	vsaUUID1 := s.newMachineVolumeAttachment(c, vsUUID1, netNodeUUID)
	vsaUUID2 := s.newMachineVolumeAttachment(c, vsUUID2, netNodeUUID)
	vsaUUID3 := s.newMachineVolumeAttachment(c, vsUUID3, netNodeUUID)

	st := NewState(s.TxnRunnerFactory())
	lives, err := st.GetVolumeAttachmentLifeForNetNode(
		c.Context(), domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(err, tc.ErrorIsNil)
	c.Check(lives, tc.DeepEquals, map[string]domainlife.Life{
		vsaUUID1: domainlife.Alive,
		vsaUUID2: domainlife.Alive,
		vsaUUID3: domainlife.Alive,
	})

	// Apply a life change to one of the attachments and check the change comes
	// out.
	s.changeVolumeAttachmentLife(c, vsaUUID1, domainlife.Dying)
	lives, err = st.GetVolumeAttachmentLifeForNetNode(
		c.Context(), domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(err, tc.ErrorIsNil)
	c.Check(lives, tc.DeepEquals, map[string]domainlife.Life{
		vsaUUID1: domainlife.Dying,
		vsaUUID2: domainlife.Alive,
		vsaUUID3: domainlife.Alive,
	})
}

// TestGetVolumeAttachmentLifeNoResults tests that when no attachment lives
// exist for a net node an empty result is returned with no error.
func (s *volumeSuite) TestGetVolumeAttachmentLifeNoResults(c *tc.C) {
	netNodeUUID := s.newNetNode(c)
	st := NewState(s.TxnRunnerFactory())
	lives, err := st.GetVolumeAttachmentLifeForNetNode(
		c.Context(), domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(err, tc.ErrorIsNil)
	c.Check(lives, tc.HasLen, 0)
}

// TestGetVolumeAttachmentPlanLifeFornetNode tests that the correct life is
// reported for each volume attachment plan associated with the given net node.
// We expect in this test that it is the volume id for the attachment plan
// that is returned and not the uuid for the attachment plan.
//
// We also inject a life change during the test to make sure that it is
// reflected.
func (s *volumeSuite) TestGetVolumeAttachmentPlanLifeForNetNode(c *tc.C) {
	netNodeUUID := s.newNetNode(c)
	vsOneUUID, vOneID := s.newMachineVolume(c)
	vsTwoUUID, vTwoID := s.newMachineVolume(c)
	vsThreeUUID, vThreeID := s.newMachineVolume(c)
	vsapOneUUID := s.newVolumeAttachmentPlan(c, vsOneUUID, netNodeUUID)
	s.newVolumeAttachmentPlan(c, vsTwoUUID, netNodeUUID)
	s.newVolumeAttachmentPlan(c, vsThreeUUID, netNodeUUID)

	st := NewState(s.TxnRunnerFactory())
	lives, err := st.GetVolumeAttachmentPlanLifeForNetNode(
		c.Context(), domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(err, tc.ErrorIsNil)
	c.Check(lives, tc.DeepEquals, map[string]domainlife.Life{
		vOneID:   domainlife.Alive,
		vTwoID:   domainlife.Alive,
		vThreeID: domainlife.Alive,
	})

	// Apply a life change to one of the attachments and check the change comes
	// out.
	s.changeVolumeAttachmentPlanLife(c, vsapOneUUID, domainlife.Dying)
	lives, err = st.GetVolumeAttachmentPlanLifeForNetNode(
		c.Context(), domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(err, tc.ErrorIsNil)
	c.Check(lives, tc.DeepEquals, map[string]domainlife.Life{
		vOneID:   domainlife.Dying,
		vTwoID:   domainlife.Alive,
		vThreeID: domainlife.Alive,
	})
}

// TestGetVolumeAttachmentPlanLifeNoResults tests that when no attachment plan
// lives exist for a net node an empty result is returned with no error.
func (s *volumeSuite) TestGetVolumeAttachmentPlanLifeNoResults(c *tc.C) {
	netNodeUUID := s.newNetNode(c)
	st := NewState(s.TxnRunnerFactory())
	lives, err := st.GetVolumeAttachmentPlanLifeForNetNode(
		c.Context(), domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(err, tc.ErrorIsNil)
	c.Check(lives, tc.HasLen, 0)
}

// TestGetVolumeLifeForNetNode tests if we can get the volume life for
// volumes attached to a specified machine's net node.
func (s *volumeSuite) TestGetVolumeLifeForNetNode(c *tc.C) {
	st := NewState(s.TxnRunnerFactory())

	netNodeUUID := s.newNetNode(c)
	vsOneUUID, vsOneID := s.newMachineVolume(c)
	_ = s.newMachineVolumeAttachment(c, vsOneUUID, netNodeUUID)
	vsTwoUUID, vsTwoID := s.newMachineVolume(c)
	_ = s.newMachineVolumeAttachment(c, vsTwoUUID, netNodeUUID)
	vsThreeUUID, vsThreeID := s.newMachineVolume(c)
	_ = s.newMachineVolumeAttachment(c, vsThreeUUID, netNodeUUID)

	s.changeVolumeLife(c, vsTwoUUID, domainlife.Dying)
	s.changeVolumeLife(c, vsThreeUUID, domainlife.Dead)

	// Add unrelated volumes.
	_, _ = s.newModelVolume(c)
	vsIDOtherMachine, _ := s.newMachineVolume(c)
	_ = s.newMachineVolumeAttachment(c, vsIDOtherMachine, s.newNetNode(c))

	vsUUIDs, err := st.GetVolumeLifeForNetNode(
		c.Context(), domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(vsUUIDs, tc.DeepEquals, map[string]domainlife.Life{
		vsOneID:   domainlife.Alive,
		vsTwoID:   domainlife.Dying,
		vsThreeID: domainlife.Dead,
	})
}

// TestInitialWatchStatementMachineProvisionedVolumes tests the initial query
// for machine provisioned volumes watcher returns only the volume UUIDs
// attached to the specified machine net node.
func (s *volumeSuite) TestInitialWatchStatementMachineProvisionedVolumes(c *tc.C) {
	st := NewState(s.TxnRunnerFactory())

	netNodeUUID := s.newNetNode(c)
	vsOneUUID, vsOneID := s.newMachineVolume(c)
	_ = s.newMachineVolumeAttachment(c, vsOneUUID, netNodeUUID)
	vsTwoUUID, vsTwoID := s.newMachineVolume(c)
	_ = s.newMachineVolumeAttachment(c, vsTwoUUID, netNodeUUID)

	// Add unrelated volumes.
	_, _ = s.newModelVolume(c)
	vsIDOtherMachine, _ := s.newMachineVolume(c)
	_ = s.newMachineVolumeAttachment(c, vsIDOtherMachine, s.newNetNode(c))

	ns, initialQuery := st.InitialWatchStatementMachineProvisionedVolumes(
		domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(ns, tc.Equals, "storage_volume_life_machine_provisioning")

	db := s.TxnRunner()
	vsUUIDs, err := initialQuery(c.Context(), db)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(vsUUIDs, tc.DeepEquals, map[string]domainlife.Life{
		vsOneID: domainlife.Alive,
		vsTwoID: domainlife.Alive,
	})
}

// TestInitialWatchStatementMachineProvisionedVolumesNone tests the initial
// query for machine provisioned volumes watcher does not return an error
// when no machine provisioned volumes are attached to the specified machine
// net node.
func (s *volumeSuite) TestInitialWatchStatementMachineProvisionedVolumesNone(c *tc.C) {
	st := NewState(s.TxnRunnerFactory())

	netNodeUUID := s.newNetNode(c)

	// Add unrelated volumes.
	_, _ = s.newModelVolume(c)
	vsIDOtherMachine, _ := s.newMachineVolume(c)
	s.newMachineVolumeAttachment(c, vsIDOtherMachine, s.newNetNode(c))

	ns, initialQuery := st.InitialWatchStatementMachineProvisionedVolumes(
		domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(ns, tc.Equals, "storage_volume_life_machine_provisioning")

	db := s.TxnRunner()
	vsUUIDs, err := initialQuery(c.Context(), db)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(vsUUIDs, tc.HasLen, 0)
}

// TestInitialWatchStatementMachineProvisionedVolumesNetNodeMissing tests
// the initial query for machine provisioned volumes watcher errors when the
// net node specified is not found.
func (s *volumeSuite) TestInitialWatchStatementMachineProvisionedVolumesNetNodeMissing(c *tc.C) {
	st := NewState(s.TxnRunnerFactory())

	netNodeUUID := uuid.MustNewUUID().String()

	ns, initialQuery := st.InitialWatchStatementMachineProvisionedVolumes(
		domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(ns, tc.Equals, "storage_volume_life_machine_provisioning")

	db := s.TxnRunner()
	_, err := initialQuery(c.Context(), db)
	c.Assert(err, tc.NotNil)
}

// TestInitialWatchStatementModelProvisionedVolumesNone tests the initial
// query for a model provisioned filsystem watcher returns no error when there
// is not any model provisioned volumes.
func (s *volumeSuite) TestInitialWatchStatementModelProvisionedVolumesNone(c *tc.C) {
	st := NewState(s.TxnRunnerFactory())
	_, _ = s.newMachineVolume(c)

	ns, initialQuery := st.InitialWatchStatementModelProvisionedVolumes()
	c.Check(ns, tc.Equals, "storage_volume_life_model_provisioning")

	db := s.TxnRunner()
	vsIDs, err := initialQuery(c.Context(), db)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(vsIDs, tc.HasLen, 0)
}

// TestInitialWatchStatementModelProvisionedVolumes tests the initial query
// for a model provisioned filsystem watcher returns only the volume IDs for
// the model provisoned volumes.
func (s *volumeSuite) TestInitialWatchStatementModelProvisionedVolumes(c *tc.C) {
	st := NewState(s.TxnRunnerFactory())
	_, vsOneID := s.newModelVolume(c)
	_, vsTwoID := s.newModelVolume(c)
	_, _ = s.newMachineVolume(c)

	ns, initialQuery := st.InitialWatchStatementModelProvisionedVolumes()
	c.Check(ns, tc.Equals, "storage_volume_life_model_provisioning")

	db := s.TxnRunner()
	vsIDs, err := initialQuery(c.Context(), db)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(vsIDs, tc.SameContents, []string{vsOneID, vsTwoID})
}

// TestInitialWatchStatementMachineProvisionedVolumeAttachments tests the
// initial query for machine provisioned volume attachments watcher returns
// only the volume attachment UUIDs attached to the specified net node.
func (s *volumeSuite) TestInitialWatchStatementMachineProvisionedVolumeAttachments(c *tc.C) {
	netNodeUUID := s.newNetNode(c)
	vsOneUUID, _ := s.newMachineVolume(c)
	vsaOneUUID := s.newMachineVolumeAttachment(c, vsOneUUID, netNodeUUID)
	vsTwoUUID, _ := s.newMachineVolume(c)
	vsaTwoUUID := s.newMachineVolumeAttachment(c, vsTwoUUID, netNodeUUID)

	// Add unrelated volumes.
	_, _ = s.newModelVolume(c)
	vsIDOtherMachine, _ := s.newMachineVolume(c)
	_ = s.newMachineVolumeAttachment(c, vsIDOtherMachine, s.newNetNode(c))

	st := NewState(s.TxnRunnerFactory())
	ns, initialQuery := st.InitialWatchStatementMachineProvisionedVolumeAttachments(
		domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(ns, tc.Equals, "storage_volume_attachment_life_machine_provisioning")

	db := s.TxnRunner()
	vsaUUIDs, err := initialQuery(c.Context(), db)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(vsaUUIDs, tc.DeepEquals, map[string]domainlife.Life{
		vsaTwoUUID: domainlife.Alive,
		vsaOneUUID: domainlife.Alive,
	})
}

// TestInitialWatchStatementMachineProvisionedVolumeAttachmentsNone tests
// the initial query for machine provisioned volume attachments watcher does
// not return an error when no machine provisioned volume attachments are
// attached to the specified net node.
func (s *volumeSuite) TestInitialWatchStatementMachineProvisionedVolumeAttachmentsNone(c *tc.C) {
	netNodeUUID := s.newNetNode(c)

	// Add unrelated volumes.
	_, _ = s.newModelVolume(c)
	vsIDOtherMachine, _ := s.newMachineVolume(c)
	s.newMachineVolumeAttachment(c, vsIDOtherMachine, s.newNetNode(c))

	st := NewState(s.TxnRunnerFactory())
	ns, initialQuery := st.InitialWatchStatementMachineProvisionedVolumeAttachments(
		domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(ns, tc.Equals, "storage_volume_attachment_life_machine_provisioning")

	db := s.TxnRunner()
	vsaUUIDs, err := initialQuery(c.Context(), db)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(vsaUUIDs, tc.HasLen, 0)
}

// TestInitialWatchStatementMachineProvisionedVolumeAttachmentsNetNodeMissing
// tests the initial query for machine provisioned volume attachmewnts
// watcher errors when the net node specified is not found.
func (s *volumeSuite) TestInitialWatchStatementMachineProvisionedVolumeAttachmentsNetNodeMissing(c *tc.C) {
	netNodeUUID, err := domainnetwork.NewNetNodeUUID()
	c.Assert(err, tc.ErrorIsNil)

	st := NewState(s.TxnRunnerFactory())
	ns, initialQuery := st.InitialWatchStatementMachineProvisionedVolumeAttachments(
		netNodeUUID,
	)
	c.Check(ns, tc.Equals, "storage_volume_attachment_life_machine_provisioning")

	db := s.TxnRunner()
	_, err = initialQuery(c.Context(), db)
	// We don't focus on what the error is as no specific error type is offered
	// as part of the contract. We just care that an error occurred.
	c.Assert(err, tc.NotNil)
}

// TestInitialWatchStatementModelProvisionedVolumeAttachmentsNone tests the
// initial query for a model provisioned filsystem attachment watcher returns no
// error when there is no model provisioned volume attachments.
func (s *volumeSuite) TestInitialWatchStatementModelProvisionedVolumeAttachmentsNone(c *tc.C) {
	// Create a machine based volume attachment to assert  this doesn't show
	// up.
	netNode := s.newNetNode(c)
	vsUUID, _ := s.newMachineVolume(c)
	s.newMachineVolumeAttachment(c, vsUUID, netNode)

	st := NewState(s.TxnRunnerFactory())
	ns, initialQuery := st.InitialWatchStatementModelProvisionedVolumeAttachments()
	c.Check(ns, tc.Equals, "storage_volume_attachment_life_model_provisioning")

	db := s.TxnRunner()
	vsaUUIDs, err := initialQuery(c.Context(), db)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(vsaUUIDs, tc.HasLen, 0)
}

// TestInitialWatchStatementModelProvisionedVolumeAttachments tests the
// initial query for a model provisioned filsystem attachment watcher returns
// only the volume attachment uuids for the model provisoned volume
// attachments.
func (s *volumeSuite) TestInitialWatchStatementModelProvisionedVolumeAttachments(c *tc.C) {
	netNodeUUID := s.newNetNode(c)
	st := NewState(s.TxnRunnerFactory())
	vsOneUUID, _ := s.newModelVolume(c)
	vsTwoUUID, _ := s.newModelVolume(c)
	vsThreeUUID, _ := s.newMachineVolume(c)
	vsaOneUUID := s.newModelVolumeAttachment(c, vsOneUUID, netNodeUUID)
	vsaTwoUUID := s.newModelVolumeAttachment(c, vsTwoUUID, netNodeUUID)
	s.newMachineVolumeAttachment(c, vsThreeUUID, netNodeUUID)

	ns, initialQuery := st.InitialWatchStatementModelProvisionedVolumeAttachments()
	c.Check(ns, tc.Equals, "storage_volume_attachment_life_model_provisioning")

	db := s.TxnRunner()
	vsaUUIDs, err := initialQuery(c.Context(), db)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(vsaUUIDs, tc.SameContents, []string{vsaOneUUID, vsaTwoUUID})
}

// TestInitialWatchStatementMachineProvisionedVolumeAttachmentPlans tests the
// initial query for machine provisioned volume attachment plans watcher returns
// only the volume ids attached to the specified net node.
func (s *volumeSuite) TestInitialWatchStatementMachineProvisionedVolumeAttachmentPlans(c *tc.C) {
	netNodeUUID := s.newNetNode(c)
	vsOneUUID, vOneID := s.newMachineVolume(c)
	s.newVolumeAttachmentPlan(c, vsOneUUID, netNodeUUID)
	vsTwoUUID, vTwoID := s.newMachineVolume(c)
	s.newVolumeAttachmentPlan(c, vsTwoUUID, netNodeUUID)

	// Add unrelated volumes.
	_, _ = s.newModelVolume(c)
	vsIDOtherMachine, _ := s.newMachineVolume(c)
	_ = s.newVolumeAttachmentPlan(c, vsIDOtherMachine, s.newNetNode(c))

	st := NewState(s.TxnRunnerFactory())
	ns, initialQuery := st.InitialWatchStatementVolumeAttachmentPlans(
		domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(ns, tc.Equals, "storage_volume_attachment_plan_life_machine_provisioning")

	db := s.TxnRunner()
	vsaUUIDs, err := initialQuery(c.Context(), db)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(vsaUUIDs, tc.DeepEquals, map[string]domainlife.Life{
		vOneID: domainlife.Alive,
		vTwoID: domainlife.Alive,
	})
}

// TestInitialWatchStatementMachineProvisionedVolumeAttachmentPlansNone tests
// the initial query for machine provisioned volume attachment plans watcher
// does not return an error when no machine provisioned volume attachments are
// attached to the specified net node.
func (s *volumeSuite) TestInitialWatchStatementMachineProvisionedVolumeAttachmentPlansNone(c *tc.C) {
	netNodeUUID := s.newNetNode(c)
	// Add unrelated volumes.
	vsUUID, _ := s.newMachineVolume(c)
	s.newVolumeAttachmentPlan(c, vsUUID, s.newNetNode(c))

	st := NewState(s.TxnRunnerFactory())
	ns, initialQuery := st.InitialWatchStatementVolumeAttachmentPlans(
		domainnetwork.NetNodeUUID(netNodeUUID),
	)
	c.Check(ns, tc.Equals, "storage_volume_attachment_plan_life_machine_provisioning")

	db := s.TxnRunner()
	planUUIDs, err := initialQuery(c.Context(), db)
	c.Assert(err, tc.ErrorIsNil)
	c.Check(planUUIDs, tc.HasLen, 0)
}

// TestInitialWatchStatementMachineProvisionedVolumeAttachmentPlansNetNodeMissing
// tests the initial query for machine provisioned volume attachmewnt plans
// watcher errors when the net node specified is not found.
func (s *volumeSuite) TestInitialWatchStatementMachineProvisionedVolumeAttachmentPlansNetNodeMissing(c *tc.C) {
	netNodeUUID, err := domainnetwork.NewNetNodeUUID()
	c.Assert(err, tc.ErrorIsNil)

	st := NewState(s.TxnRunnerFactory())
	ns, initialQuery := st.InitialWatchStatementVolumeAttachmentPlans(
		netNodeUUID,
	)
	c.Check(ns, tc.Equals, "storage_volume_attachment_plan_life_machine_provisioning")

	db := s.TxnRunner()
	_, err = initialQuery(c.Context(), db)
	// We don't focus on what the error is as no specific error type is offered
	// as part of the contract. We just care that an error occurred.
	c.Assert(err, tc.NotNil)
}
