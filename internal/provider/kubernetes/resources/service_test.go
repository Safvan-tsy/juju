// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package resources_test

import (
	"context"
	"testing"

	"github.com/juju/errors"
	"github.com/juju/tc"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juju/juju/internal/provider/kubernetes/resources"
)

type serviceSuite struct {
	resourceSuite
}

func TestServiceSuite(t *testing.T) {
	tc.Run(t, &serviceSuite{})
}

func (s *serviceSuite) TestApply(c *tc.C) {
	ds := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ds1",
			Namespace: "test",
		},
	}
	// Create.
	dsResource := resources.NewService(s.client.CoreV1().Services(ds.Namespace), "test", "ds1", ds)
	c.Assert(dsResource.Apply(context.TODO()), tc.ErrorIsNil)
	result, err := s.client.CoreV1().Services("test").Get(context.TODO(), "ds1", metav1.GetOptions{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(len(result.GetAnnotations()), tc.Equals, 0)

	// Update.
	ds.SetAnnotations(map[string]string{"a": "b"})
	dsResource = resources.NewService(s.client.CoreV1().Services(ds.Namespace), "test", "ds1", ds)
	c.Assert(dsResource.Apply(context.TODO()), tc.ErrorIsNil)

	result, err = s.client.CoreV1().Services("test").Get(context.TODO(), "ds1", metav1.GetOptions{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(result.GetName(), tc.Equals, `ds1`)
	c.Assert(result.GetNamespace(), tc.Equals, `test`)
	c.Assert(result.GetAnnotations(), tc.DeepEquals, map[string]string{"a": "b"})
}

func (s *serviceSuite) TestGet(c *tc.C) {
	template := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ds1",
			Namespace: "test",
		},
	}
	ds1 := template
	ds1.SetAnnotations(map[string]string{"a": "b"})
	_, err := s.client.CoreV1().Services("test").Create(context.TODO(), &ds1, metav1.CreateOptions{})
	c.Assert(err, tc.ErrorIsNil)

	dsResource := resources.NewService(s.client.CoreV1().Services(ds1.Namespace), "test", "ds1", &template)
	c.Assert(len(dsResource.GetAnnotations()), tc.Equals, 0)
	err = dsResource.Get(context.TODO())
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(dsResource.GetName(), tc.Equals, `ds1`)
	c.Assert(dsResource.GetNamespace(), tc.Equals, `test`)
	c.Assert(dsResource.GetAnnotations(), tc.DeepEquals, map[string]string{"a": "b"})
}

func (s *serviceSuite) TestDelete(c *tc.C) {
	ds := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ds1",
			Namespace: "test",
		},
	}
	_, err := s.client.CoreV1().Services("test").Create(context.TODO(), &ds, metav1.CreateOptions{})
	c.Assert(err, tc.ErrorIsNil)

	result, err := s.client.CoreV1().Services("test").Get(context.TODO(), "ds1", metav1.GetOptions{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(result.GetName(), tc.Equals, `ds1`)

	dsResource := resources.NewService(s.client.CoreV1().Services(ds.Namespace), "test", "ds1", &ds)
	err = dsResource.Delete(context.TODO())
	c.Assert(err, tc.ErrorIsNil)

	err = dsResource.Get(context.TODO())
	c.Assert(err, tc.Satisfies, errors.IsNotFound)

	_, err = s.client.CoreV1().Services("test").Get(context.TODO(), "ds1", metav1.GetOptions{})
	c.Assert(err, tc.Satisfies, k8serrors.IsNotFound)
}
