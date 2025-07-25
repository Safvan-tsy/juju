// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package kubernetes

import (
	"context"

	"github.com/juju/errors"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"

	"github.com/juju/juju/internal/provider/kubernetes/constants"
	"github.com/juju/juju/internal/provider/kubernetes/utils"
)

// EnsureMutatingWebhookConfiguration is used by the model operator to set up Juju's web hook.
func (k *kubernetesClient) EnsureMutatingWebhookConfiguration(ctx context.Context, cfg *admissionregistrationv1.MutatingWebhookConfiguration) (func(), error) {
	cleanUp := func() {}
	api := k.client().AdmissionregistrationV1().MutatingWebhookConfigurations()
	out, err := api.Create(ctx, cfg, metav1.CreateOptions{})
	if err == nil {
		logger.Debugf(ctx, "MutatingWebhookConfiguration %q created", out.GetName())
		cleanUp = func() {
			_ = api.Delete(ctx, out.GetName(), utils.NewPreconditionDeleteOptions(out.GetUID()))
		}
		return cleanUp, nil
	}
	if !k8serrors.IsAlreadyExists(err) {
		return cleanUp, errors.Trace(err)
	}

	existing, err := api.Get(ctx, cfg.GetName(), metav1.GetOptions{})
	if err != nil {
		return cleanUp, errors.Trace(err)
	}
	existingLabelVersion, err := utils.MatchModelMetaLabelVersion(existing.ObjectMeta, k.modelName, k.modelUUID, k.controllerUUID)
	if err != nil {
		return nil, errors.Annotatef(err, "ensuring MutatingWebhookConfiguration %q with labels %v ", cfg.GetName(), existing.Labels)
	}
	if existingLabelVersion < k.labelVersion {
		logger.Warningf(ctx, "updating label version for existing MutatingWebhookConfiguration %q from %d to %d ", cfg.GetName(), existingLabelVersion, k.labelVersion)
	}

	cfg.SetResourceVersion(existing.GetResourceVersion())
	_, err = api.Update(ctx, cfg, metav1.UpdateOptions{})
	logger.Debugf(ctx, "updating MutatingWebhookConfiguration %q", cfg.GetName())
	return cleanUp, errors.Trace(err)
}

func (k *kubernetesClient) listMutatingWebhookConfigurations(ctx context.Context, selector k8slabels.Selector) ([]admissionregistrationv1.MutatingWebhookConfiguration, error) {
	listOps := metav1.ListOptions{
		LabelSelector: selector.String(),
	}
	cfgList, err := k.client().AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, listOps)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(cfgList.Items) == 0 {
		return nil, errors.NotFoundf("MutatingWebhookConfiguration with selector %q", selector)
	}
	return cfgList.Items, nil
}

func (k *kubernetesClient) deleteMutatingWebhookConfigurations(ctx context.Context, selector k8slabels.Selector) error {
	err := k.client().AdmissionregistrationV1().MutatingWebhookConfigurations().DeleteCollection(ctx, metav1.DeleteOptions{
		PropagationPolicy: constants.DefaultPropagationPolicy(),
	}, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return errors.Trace(err)
}

func (k *kubernetesClient) listValidatingWebhookConfigurations(ctx context.Context, selector k8slabels.Selector) ([]admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	listOps := metav1.ListOptions{
		LabelSelector: selector.String(),
	}
	cfgList, err := k.client().AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, listOps)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(cfgList.Items) == 0 {
		return nil, errors.NotFoundf("ValidatingWebhookConfiguration with selector %q", selector)
	}
	return cfgList.Items, nil
}

func (k *kubernetesClient) deleteValidatingWebhookConfigurations(ctx context.Context, selector k8slabels.Selector) error {
	err := k.client().AdmissionregistrationV1().ValidatingWebhookConfigurations().DeleteCollection(ctx, metav1.DeleteOptions{
		PropagationPolicy: constants.DefaultPropagationPolicy(),
	}, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return errors.Trace(err)
}
