// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package resources_test

import (
	"fmt"
	"testing"

	"github.com/juju/errors"
	"github.com/juju/tc"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juju/juju/internal/provider/kubernetes/resources"
)

type persistentVolumeClaimSuite struct {
	resourceSuite
}

func TestPersistentVolumeClaimSuite(t *testing.T) {
	tc.Run(t, &persistentVolumeClaimSuite{})
}

func (s *persistentVolumeClaimSuite) TestApply(c *tc.C) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc1",
			Namespace: "test",
		},
	}
	// Create.
	pvcResource := resources.NewPersistentVolumeClaim(s.client.CoreV1().PersistentVolumeClaims(pvc.Namespace), "test", "pvc1", pvc)
	c.Assert(pvcResource.Apply(c.Context()), tc.ErrorIsNil)
	result, err := s.client.CoreV1().PersistentVolumeClaims("test").Get(c.Context(), "pvc1", metav1.GetOptions{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(len(result.GetAnnotations()), tc.Equals, 0)

	// Update.
	pvc.SetAnnotations(map[string]string{"a": "b"})
	pvcResource = resources.NewPersistentVolumeClaim(s.client.CoreV1().PersistentVolumeClaims(pvc.Namespace), "test", "pvc1", pvc)
	c.Assert(pvcResource.Apply(c.Context()), tc.ErrorIsNil)

	result, err = s.client.CoreV1().PersistentVolumeClaims("test").Get(c.Context(), "pvc1", metav1.GetOptions{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(result.GetName(), tc.Equals, `pvc1`)
	c.Assert(result.GetNamespace(), tc.Equals, `test`)
	c.Assert(result.GetAnnotations(), tc.DeepEquals, map[string]string{"a": "b"})
}

func (s *persistentVolumeClaimSuite) TestGet(c *tc.C) {
	template := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc1",
			Namespace: "test",
		},
	}
	pvc1 := template
	pvc1.SetAnnotations(map[string]string{"a": "b"})
	_, err := s.client.CoreV1().PersistentVolumeClaims("test").Create(c.Context(), &pvc1, metav1.CreateOptions{})
	c.Assert(err, tc.ErrorIsNil)

	pvcResource := resources.NewPersistentVolumeClaim(s.client.CoreV1().PersistentVolumeClaims(pvc1.Namespace), "test", "pvc1", &template)
	c.Assert(len(pvcResource.GetAnnotations()), tc.Equals, 0)
	err = pvcResource.Get(c.Context())
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(pvcResource.GetName(), tc.Equals, `pvc1`)
	c.Assert(pvcResource.GetNamespace(), tc.Equals, `test`)
	c.Assert(pvcResource.GetAnnotations(), tc.DeepEquals, map[string]string{"a": "b"})
}

func (s *persistentVolumeClaimSuite) TestDelete(c *tc.C) {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc1",
			Namespace: "test",
		},
	}
	_, err := s.client.CoreV1().PersistentVolumeClaims("test").Create(c.Context(), &pvc, metav1.CreateOptions{})
	c.Assert(err, tc.ErrorIsNil)

	result, err := s.client.CoreV1().PersistentVolumeClaims("test").Get(c.Context(), "pvc1", metav1.GetOptions{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(result.GetName(), tc.Equals, `pvc1`)

	pvcResource := resources.NewPersistentVolumeClaim(s.client.CoreV1().PersistentVolumeClaims(pvc.Namespace), "test", "pvc1", &pvc)
	err = pvcResource.Delete(c.Context())
	c.Assert(err, tc.ErrorIsNil)

	err = pvcResource.Delete(c.Context())
	c.Assert(err, tc.ErrorIs, errors.NotFound)

	err = pvcResource.Get(c.Context())
	c.Assert(err, tc.Satisfies, errors.IsNotFound)

	_, err = s.client.CoreV1().PersistentVolumeClaims("test").Get(c.Context(), "pvc1", metav1.GetOptions{})
	c.Assert(err, tc.Satisfies, k8serrors.IsNotFound)
}

func (s *persistentVolumeClaimSuite) TestList(c *tc.C) {
	// Unfortunately with the K8s fake/testing API there doesn't seem to be a
	// way to call List multiple times with "Continue" set.

	// Create fake persistent volume claims, some of which have a label
	for i := 0; i < 7; i++ {
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("pvc%d", i),
				Namespace: "test",
			},
		}
		if i%3 == 0 {
			pvc.ObjectMeta.Labels = map[string]string{"modulo": "three"}
		}
		_, err := s.client.CoreV1().PersistentVolumeClaims("test").Create(c.Context(), &pvc, metav1.CreateOptions{})
		c.Assert(err, tc.ErrorIsNil)
	}

	// List PVCs filtered by the label
	listed, err := resources.ListPersistentVolumeClaims(c.Context(), s.client, "test", metav1.ListOptions{
		LabelSelector: "modulo == three",
	})
	c.Assert(err, tc.ErrorIsNil)

	// Check that we fetch the right ones
	c.Assert(len(listed), tc.Equals, 3)
	for i, pvc := range listed {
		c.Assert(pvc.Name, tc.Equals, fmt.Sprintf("pvc%d", i*3))
		c.Assert(pvc.Labels, tc.DeepEquals, map[string]string{"modulo": "three"})
	}
}
