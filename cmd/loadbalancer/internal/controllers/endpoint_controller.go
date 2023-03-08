/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package loadbalancer

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/config"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	AnnotationKeyEndpointsVersion = "service.kubernetes.io/kubeblocks-loadbalancer-endpoints-version"
)

var endpointsFilterPredicate = func(object client.Object) bool {
	for k, v := range config.EndpointsLabels {
		if object.GetLabels()[k] != v {
			return false
		}
	}
	return true
}

type EndpointController struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	logger logr.Logger
}

func NewEndpointController(logger logr.Logger, client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder) (*EndpointController, error) {
	return &EndpointController{
		logger:   logger,
		Client:   client,
		Scheme:   scheme,
		Recorder: recorder,
	}, nil
}

func (c *EndpointController) Start(ctx context.Context) error {
	return nil
}

func (c *EndpointController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxLog := c.logger.WithValues("endpoints", req.NamespacedName.String())
	ctxLog.Info("Receive endpoint reconcile event")

	endpoints := &corev1.Endpoints{}
	if err := c.Client.Get(ctx, req.NamespacedName, endpoints); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, c.logger, "")
	}

	listOptions := []client.ListOption{client.InNamespace(endpoints.GetNamespace())}
	for k, v := range endpoints.GetObjectMeta().GetLabels() {
		listOptions = append(listOptions, client.MatchingLabels{k: v})
	}

	services := &corev1.ServiceList{}
	if err := c.Client.List(ctx, services, listOptions...); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, c.logger, "")
	}

	for i := range services.Items {
		service := &services.Items[i]
		if _, ok := service.Annotations[AnnotationKeyLoadBalancerType]; !ok {
			ctxLog.Info("Ignore unrelated endpoints")
			return intctrlutil.Reconciled()
		}

		// endpoint changed, trigger service reconcile
		annotations := service.Annotations
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[AnnotationKeyEndpointsVersion] = endpoints.GetObjectMeta().GetResourceVersion()
		service.SetAnnotations(annotations)
		if err := c.Client.Update(ctx, service); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, c.logger, "")
		}
		ctxLog.Info("Successfully updated service")
	}

	return intctrlutil.Reconciled()
}

func (c *EndpointController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).WithOptions(controller.Options{
		MaxConcurrentReconciles: config.MaxConcurrentReconciles,
	}).For(&corev1.Endpoints{}, builder.WithPredicates(predicate.NewPredicateFuncs(endpointsFilterPredicate))).Complete(c)
}
