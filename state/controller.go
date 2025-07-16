// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"github.com/juju/errors"
	"github.com/juju/mgo/v3"
	"github.com/juju/mgo/v3/bson"
	"github.com/juju/names/v6"
)

// Controller encapsulates state for the Juju controller as a whole,
// as opposed to model specific functionality.
//
// This type is primarily used in the state.Initialize function, and
// in the yet to be hooked up controller worker.
type Controller struct {
	pool     *StatePool
	ownsPool bool
}

// NewController returns a controller object that doesn't own
// the state pool it has been given. This is for convenience
// at this time to get access to controller methods.
func NewController(pool *StatePool) *Controller {
	return &Controller{pool: pool}
}

// StatePool provides access to the state pool of the controller.
func (ctlr *Controller) StatePool() *StatePool {
	return ctlr.pool
}

// SystemState returns the State object for the controller model.
func (ctlr *Controller) SystemState() (*State, error) {
	return ctlr.pool.SystemState()
}

// Close the connection to the database.
func (ctlr *Controller) Close() error {
	if ctlr.ownsPool {
		ctlr.pool.Close()
	}
	return nil
}

// GetState returns a new State instance for the specified model. The
// connection uses the same credentials and policy as the Controller.
func (ctlr *Controller) GetState(modelTag names.ModelTag) (*PooledState, error) {
	return ctlr.pool.Get(modelTag.Id())
}

type controllersDoc struct {
	Id            string   `bson:"_id"`
	CloudName     string   `bson:"cloud"`
	ModelUUID     string   `bson:"model-uuid"`
	ControllerIds []string `bson:"controller-ids"`
}

// ControllerInfo holds information about currently
// configured controller machines.
type ControllerInfo struct {
	// CloudName is the name of the cloud to which this controller is deployed.
	CloudName string

	// ModelTag identifies the initial model. Only the initial
	// model is able to have machines that manage state. The initial
	// model is the model that is created when bootstrapping.
	ModelTag names.ModelTag

	// ControllerIds holds the ids of all the controller nodes.
	// It's main purpose is to allow assertions tha the set of
	// controllers hasn't changed when adding/removing controller nodes.
	ControllerIds []string
}

// ControllerInfo returns information about
// the currently configured controller machines.
func (st *State) ControllerInfo() (*ControllerInfo, error) {
	session := st.session.Copy()
	defer session.Close()
	return readRawControllerInfo(session)
}

// readRawControllerInfo reads ControllerInfo direct from the supplied session,
// falling back to the bootstrap model document to extract the UUID when
// required.
func readRawControllerInfo(session *mgo.Session) (*ControllerInfo, error) {
	db := session.DB(jujuDB)
	controllers := db.C(controllersC)

	var doc controllersDoc
	err := controllers.Find(bson.D{{"_id", modelGlobalKey}}).One(&doc)
	if err == mgo.ErrNotFound {
		return nil, errors.NotFoundf("controllers document")
	}
	if err != nil {
		return nil, errors.Annotatef(err, "cannot get controllers document")
	}
	return &ControllerInfo{
		CloudName:     doc.CloudName,
		ModelTag:      names.NewModelTag(doc.ModelUUID),
		ControllerIds: doc.ControllerIds,
	}, nil
}

// sshServerHostKeyKeyDocId holds the document ID to retrieve the
// host key within the controller configuration collection.
const sshServerHostKeyDocId = "sshServerHostKey"

// sshServerHostKeyDoc holds the host key for the SSH server.
type sshServerHostKeyDoc struct {
	Key string `bson:"key"`
}

// SSHServerHostKey returns the host key for the SSH server. This key was set
// during the controller bootstrap process via bootstrap-state and is currently
// a FIXED value.
func (st *State) SSHServerHostKey() (string, error) {
	controllers, closer := st.db().GetCollection(controllersC)
	defer closer()

	var keyDoc sshServerHostKeyDoc
	err := controllers.Find(bson.D{{"_id", sshServerHostKeyDocId}}).One(&keyDoc)
	if err != nil {
		return "", errors.Trace(err)
	}
	return keyDoc.Key, nil
}
