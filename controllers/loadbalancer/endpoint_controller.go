package loadbalancer

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

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

func (c *EndpointController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxLog := c.logger.WithValues("endpoints", req.NamespacedName.String())
	ctxLog.Info("Receive endpoint reconcile event", "name", req.NamespacedName)

	endpoints := &corev1.Endpoints{}
	if err := c.Client.Get(ctx, req.NamespacedName, endpoints); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, c.logger, "")
	}

	listOptions := []client.ListOption{client.InNamespace(endpoints.GetNamespace())}
	for k, v := range endpoints.GetObjectMeta().GetLabels() {
		listOptions = append(listOptions, client.MatchingLabels{k: v})
	}

	service := &corev1.Service{}
	if err := c.Client.Get(ctx, req.NamespacedName, service); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, c.logger, "")
	}

	if _, ok := service.Annotations[AnnotationKeyLoadBalancerType]; !ok {
		ctxLog.Info("Ignore unrelated endpoints")
		return intctrlutil.Reconciled()
	}

	// endpoint changed, trigger service reconcile
	service.SetGeneration(service.GetGeneration() + 1)
	if err := c.Client.Update(ctx, service); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, c.logger, "")
	}
	ctxLog.Info("Successfully updated service generation")

	return intctrlutil.Reconciled()
}

func (c *EndpointController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&corev1.Endpoints{}).Complete(c)
}
