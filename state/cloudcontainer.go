// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"github.com/juju/errors"
	"github.com/juju/mgo/v3/bson"
)

// CloudContainer represents the state of a CAAS container, eg pod.
type CloudContainer interface {
	// Unit returns the name of the unit for this container.
	Unit() string

	// ProviderId returns the id assigned to the container/pod
	// by the cloud.
	ProviderId() string

	// Ports returns the open container ports.
	Ports() []string
}

// cloudContainer is an implementation of CloudContainer.
type cloudContainer struct {
	doc      cloudContainerDoc
	unitName string
}

type cloudContainerDoc struct {
	// Id holds cloud container document key.
	// It is the global key of the unit represented
	// by this container.
	Id string `bson:"_id"`

	ProviderId string   `bson:"provider-id"`
	Ports      []string `bson:"ports"`
}

// Id implements CloudContainer.
func (c *cloudContainer) Id() string {
	return c.doc.Id
}

// Unit implements CloudContainer.
func (c *cloudContainer) Unit() string {
	return c.unitName
}

// ProviderId implements CloudContainer.
func (c *cloudContainer) ProviderId() string {
	return c.doc.ProviderId
}

// Ports implements CloudContainer.
func (c *cloudContainer) Ports() []string {
	return c.doc.Ports
}

// Containers returns the containers for the specified provider ids.
func (m *CAASModel) Containers(providerIds ...string) ([]CloudContainer, error) {
	coll, closer := m.st.db().GetCollection(cloudContainersC)
	defer closer()

	var all []cloudContainerDoc
	err := coll.Find(bson.D{{"provider-id", bson.D{{"$in", providerIds}}}}).All(&all)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var result []CloudContainer
	for _, doc := range all {
		unitKey := m.localID(doc.Id)
		// key is "u#<unitname>#charm"
		idx := len(unitKey) - len("#charm")
		result = append(result, &cloudContainer{doc: doc, unitName: unitKey[2:idx]})
	}
	return result, nil
}
