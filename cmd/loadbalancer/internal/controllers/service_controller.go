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
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/agent"
	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/cloud"
	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/config"
	pb "github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/protocol"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	FinalizerKey = "service.kubernetes.io/kubeblocks-loadbalancer-finalizer"

	AnnotationKeyENIId        = "service.kubernetes.io/kubeblocks-loadbalancer-eni-id"
	AnnotationKeyENINodeIP    = "service.kubernetes.io/kubeblocks-loadbalancer-eni-node-ip"
	AnnotationKeyFloatingIP   = "service.kubernetes.io/kubeblocks-loadbalancer-floating-ip"
	AnnotationKeySubnetID     = "service.kubernetes.io/kubeblocks-loadbalancer-subnet-id"
	AnnotationKeyMasterNodeIP = "service.kubernetes.io/kubeblocks-loadbalancer-master-node-ip"

	AnnotationKeyLoadBalancerType            = "service.kubernetes.io/kubeblocks-loadbalancer-type"
	AnnotationValueLoadBalancerTypePrivateIP = "private-ip"
	AnnotationValueLoadBalancerTypeNone      = "none"

	AnnotationKeyTrafficPolicy                  = "service.kubernetes.io/kubeblocks-loadbalancer-traffic-policy"
	AnnotationValueClusterTrafficPolicy         = "Cluster"
	AnnotationValueLocalTrafficPolicy           = "Local"
	AnnotationValueBestEffortLocalTrafficPolicy = "BestEffortLocal"
	DefaultTrafficPolicy                        = AnnotationValueClusterTrafficPolicy
)

var serviceFilterPredicate = func(object client.Object) bool {
	for k, v := range config.ServiceLabels {
		if object.GetLabels()[k] != v {
			return false
		}
	}
	return true
}

type FloatingIP struct {
	ip       string
	subnetID string
	nodeIP   string
	eni      *pb.ENIMetadata
}

type ServiceController struct {
	sync.RWMutex
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	logger   logr.Logger
	cp       cloud.Provider
	nm       agent.NodeManager
	tps      map[string]TrafficPolicy
	cache    map[string]*FloatingIP
}

func NewServiceController(logger logr.Logger, client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder, cp cloud.Provider, nm agent.NodeManager) (*ServiceController, error) {
	c := &ServiceController{
		Client:   client,
		Scheme:   scheme,
		Recorder: recorder,
		logger:   logger,
		cp:       cp,
		nm:       nm,
		cache:    make(map[string]*FloatingIP),
	}

	c.initTrafficPolicies()

	return c, nil
}

func (c *ServiceController) Start(ctx context.Context) error {
	nodeList, err := c.nm.GetNodes()
	if err != nil {
		return errors.Wrap(err, "Failed to get cluster nodes")
	}
	if err := c.initNodes(nodeList); err != nil {
		return errors.Wrap(err, "Failed to init nodes")
	}
	return nil
}

func (c *ServiceController) initTrafficPolicies() {
	var (
		cp  = &ClusterTrafficPolicy{nm: c.nm}
		lp  = &LocalTrafficPolicy{logger: c.logger, nm: c.nm, client: c.Client}
		blp = &BestEffortLocalPolicy{LocalTrafficPolicy: *lp, ClusterTrafficPolicy: *cp}
	)
	c.tps = map[string]TrafficPolicy{
		AnnotationValueClusterTrafficPolicy:         cp,
		AnnotationValueLocalTrafficPolicy:           lp,
		AnnotationValueBestEffortLocalTrafficPolicy: blp,
	}
}

func (c *ServiceController) initNodes(nodeList []agent.Node) error {
	for _, item := range nodeList {
		if err := c.initNode(item.GetIP()); err != nil {
			return errors.Wrapf(err, "Failed to init node %s", item.GetIP())
		}
	}
	return nil
}

func (c *ServiceController) initNode(nodeIP string) error {
	ctxLog := c.logger.WithValues("node", nodeIP)

	node, err := c.nm.GetNode(nodeIP)
	if err != nil {
		return errors.Wrapf(err, "Failed to get rpc client for node %s", nodeIP)
	}
	enis, err := node.GetManagedENIs()
	if err != nil {
		return errors.Wrapf(err, "Failed to query enis from node %s", nodeIP)
	}
	for _, eni := range enis {
		for _, addr := range eni.Ipv4Addresses {
			if err := node.SetupNetworkForService(addr.Address, eni); err != nil {
				return errors.Wrapf(err, "Failed to init service private ip %s for node %s", addr.Address, nodeIP)
			}
			c.setFloatingIP(addr.Address, &FloatingIP{ip: addr.Address, subnetID: eni.SubnetId, nodeIP: nodeIP, eni: eni})
			ctxLog.Info("Successfully init service", "private ip", addr.Address)
		}
	}
	return nil
}

func (c *ServiceController) getFloatingIP(ip string) *FloatingIP {
	c.RLock()
	defer c.RUnlock()
	fip := c.cache[ip]
	if fip != nil {
		c.logger.Info("Get floating ip from cache", "ip", ip, "eni id", fip.eni.EniId)
	}
	return fip
}

func (c *ServiceController) setFloatingIP(ip string, fip *FloatingIP) {
	c.Lock()
	defer c.Unlock()
	c.cache[ip] = fip
	c.logger.Info("Put floating ip to cache", "ip", ip, "eni id", fip.eni.EniId)
}

func (c *ServiceController) removeFloatingIP(ip string) {
	c.Lock()
	defer c.Unlock()
	fip := c.cache[ip]
	delete(c.cache, ip)
	if fip != nil {
		c.logger.Info("Remove floating ip from cache", "ip", ip, "eni id", fip.eni.EniId)
	}
}

func (c *ServiceController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).WithOptions(controller.Options{
		MaxConcurrentReconciles: config.MaxConcurrentReconciles,
	}).For(&corev1.Service{}, builder.WithPredicates(predicate.NewPredicateFuncs(serviceFilterPredicate))).Complete(c)
}

func (c *ServiceController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxLog := c.logger.WithValues("service", req.NamespacedName.String())
	ctxLog.Info("Receive service reconcile event")

	svc := &corev1.Service{}
	if err := c.Client.Get(ctx, req.NamespacedName, svc); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, c.logger, "")
	}

	annotations := svc.GetAnnotations()
	if annotations[AnnotationKeyLoadBalancerType] != AnnotationValueLoadBalancerTypePrivateIP {
		ctxLog.Info("Ignore unrelated service")
		return intctrlutil.Reconciled()
	}

	if err := c.ensureFloatingIP(ctx, ctxLog, svc); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, ctxLog, "")
	}

	return intctrlutil.Reconciled()
}

func (c *ServiceController) ensureFloatingIP(ctx context.Context, ctxLog logr.Logger, svc *corev1.Service) error {
	var (
		err         error
		annotations = svc.GetAnnotations()
		fip         = c.getFloatingIP(annotations[AnnotationKeyFloatingIP])
	)

	// service is in deleting
	if !svc.GetDeletionTimestamp().IsZero() {
		return c.deleteFloatingIP(ctx, ctxLog, fip, svc)
	}

	nodeIP, err := c.chooseTrafficNode(svc)
	if err != nil {
		return errors.Wrap(err, "Failed to get master nodes")
	}

	// service is in creating
	if fip == nil {
		return c.createFloatingIP(ctx, ctxLog, nodeIP, svc)
	}

	return c.migrateFloatingIP(ctx, ctxLog, nodeIP, fip, svc)
}

func (c *ServiceController) migrateFloatingIP(ctx context.Context, ctxLog logr.Logger, nodeIP string, fip *FloatingIP, svc *corev1.Service) error {
	ctxLog = ctxLog.WithName("migrateFloatingIP").WithValues("new node", nodeIP, "old node", fip.nodeIP)

	if fip.nodeIP == nodeIP && fip.eni.EniId == svc.GetAnnotations()[AnnotationKeyENIId] {
		ctxLog.Info("Floating ip is in sync, do nothing")
		return nil
	}

	var (
		wg   = sync.WaitGroup{}
		errs []error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs = append(errs, c.migrateOnNewMaster(ctx, ctxLog, nodeIP, fip, svc))
	}()
	go func() {
		defer wg.Done()
		errs = append(errs, c.migrateOnOldMaster(ctx, ctxLog, fip))
	}()
	wg.Wait()

	var messages []string
	for _, err := range errs {
		if err == nil {
			continue
		}
		messages = append(messages, err.Error())
	}
	if len(messages) != 0 {
		return errors.New(fmt.Sprintf("Failed to migrate floating ip, err: %s", strings.Join(messages, " | ")))
	}
	return nil
}

func (c *ServiceController) migrateOnNewMaster(ctx context.Context, ctxLog logr.Logger, nodeIP string, fip *FloatingIP, svc *corev1.Service) error {
	ctxLog = ctxLog.WithName("migrateOnNewMaster").WithValues("node", nodeIP)

	if err := c.cp.DeallocIPAddresses(fip.eni.EniId, []string{fip.ip}); err != nil {
		return errors.Wrapf(err, "Failed to dealloc private ip %s", fip.ip)
	}
	ctxLog.Info("Successfully released floating ip")

	node, err := c.nm.GetNode(nodeIP)
	if err != nil {
		return errors.Wrap(err, "Failed to get master node")
	}

	newENI, err := c.tryAssignPrivateIP(ctxLog, fip.ip, node)
	if err != nil {
		return errors.Wrap(err, "Failed to assign private ip")
	}
	newFip := &FloatingIP{ip: fip.ip, subnetID: fip.subnetID, nodeIP: nodeIP, eni: newENI}
	c.setFloatingIP(newFip.ip, newFip)

	if err = node.SetupNetworkForService(newFip.ip, newENI); err != nil {
		return errors.Wrap(err, "Failed to setup host network stack for service")
	}
	ctxLog.Info("Successfully setup service network")

	if err = c.updateService(ctx, ctxLog, svc, newFip, false); err != nil {
		return errors.Wrap(err, "Failed to update service")
	}
	return nil
}

func (c *ServiceController) migrateOnOldMaster(ctx context.Context, ctxLog logr.Logger, fip *FloatingIP) error {
	ctxLog = ctxLog.WithName("migrateOnOldMaster").WithValues("node", fip.nodeIP)

	node, err := c.nm.GetNode(fip.nodeIP)
	if err != nil {
		return errors.Wrap(err, "Failed to get old master node")
	}
	if err := node.CleanNetworkForService(fip.ip, fip.eni); err != nil {
		return errors.Wrap(err, "Failed to cleanup private ip")
	}
	ctxLog.Info("Successfully clean service network ")
	return nil
}

func (c *ServiceController) createFloatingIP(ctx context.Context, ctxLog logr.Logger, nodeIP string, svc *corev1.Service) error {
	ctxLog = ctxLog.WithName("createFloatingIP").WithValues("node", nodeIP)

	node, err := c.nm.GetNode(nodeIP)
	if err != nil {
		return errors.Wrap(err, "Failed to get master node")
	}

	privateIP, eni, err := c.tryAllocPrivateIP(ctxLog, node)
	if err != nil {
		return errors.Wrap(err, "Failed to alloc new private ip for service")
	}
	ctxLog.Info("Successfully alloc private ip")

	fip := &FloatingIP{
		ip:       privateIP,
		subnetID: eni.SubnetId,
		nodeIP:   nodeIP,
		eni:      eni,
	}
	c.setFloatingIP(privateIP, fip)

	if err = node.SetupNetworkForService(fip.ip, fip.eni); err != nil {
		return errors.Wrap(err, "Failed to setup host network stack for service")
	}
	ctxLog.Info("Successfully setup service host network")

	if err = c.updateService(ctx, ctxLog, svc, fip, false); err != nil {
		return errors.Wrap(err, "Failed to update service")
	}

	return nil
}

func (c *ServiceController) deleteFloatingIP(ctx context.Context, ctxLog logr.Logger, fip *FloatingIP, svc *corev1.Service) error {
	if fip == nil {
		fip = c.buildFIPFromAnnotation(svc)
		c.logger.Info("Can not find fip in cache, use annotation", "info", fip)
	}
	ctxLog = ctxLog.WithName("deleteFloatingIP").WithValues("ip", fip.ip, "node", fip.nodeIP)

	annotations := svc.GetAnnotations()
	eniID, ok := annotations[AnnotationKeyENIId]
	if !ok {
		return errors.New("Invalid service, private ip exists but eni id not found")
	}

	ctxLog.Info("Deleting service private ip", "eni id", eniID, "ip", fip.ip)
	if err := c.cp.DeallocIPAddresses(eniID, []string{fip.ip}); err != nil {
		return errors.Wrapf(err, "Failed to dealloc private ip address %s", fip.ip)
	}
	ctxLog.Info("Successfully released floating ip")

	node, err := c.nm.GetNode(fip.nodeIP)
	if err != nil {
		return errors.Wrapf(err, "Failed to get rpc client for node %s", fip.nodeIP)
	}
	if err = node.CleanNetworkForService(fip.ip, fip.eni); err != nil {
		return errors.Wrap(err, "Failed to cleanup private ip")
	}
	ctxLog.Info("Successfully clean service network on node")

	c.removeFloatingIP(fip.ip)
	ctxLog.Info("Successfully remove floating ip from cache")

	if err = c.updateService(ctx, ctxLog, svc, fip, true); err != nil {
		return errors.Wrap(err, "Failed to update service")
	}
	ctxLog.Info("Successfully deleted floating ip and cleaned it's host networking")
	return nil

}

func (c *ServiceController) updateService(ctx context.Context, logger logr.Logger, svc *corev1.Service, fip *FloatingIP, deleting bool) error {
	annotations := svc.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if fip.eni.EniId == "" {
		return errors.New("Invalid eni id")
	}
	annotations[AnnotationKeyENIId] = fip.eni.EniId

	if fip.nodeIP == "" {
		return errors.New("Invalid node ip")
	}
	annotations[AnnotationKeyENINodeIP] = fip.nodeIP

	if fip.ip == "" {
		return errors.New("Invalid floating ip")
	}
	annotations[AnnotationKeyFloatingIP] = fip.ip

	if fip.subnetID == "" {
		return errors.New("Invalid subnet id")
	}
	annotations[AnnotationKeySubnetID] = fip.subnetID

	svc.SetAnnotations(annotations)

	if deleting {
		controllerutil.RemoveFinalizer(svc, FinalizerKey)
	} else {
		controllerutil.AddFinalizer(svc, FinalizerKey)
	}

	svc.Spec.ExternalIPs = []string{fip.ip}

	if err := c.Client.Update(ctx, svc); err != nil {
		return err
	}
	logger.Info("Successfully update service", "info", svc.String())
	return nil
}

func (c *ServiceController) tryAllocPrivateIP(ctxLog logr.Logger, node agent.Node) (string, *pb.ENIMetadata, error) {
	ctxLog = ctxLog.WithName("tryAllocPrivateIP")

	eni, err := node.ChooseENI()
	if err != nil {
		return "", nil, errors.Wrap(err, "Failed to choose busiest ENI")
	}
	ctxLog.Info("Successfully choose busiest eni", "eni id", eni.EniId)

	ip, err := c.cp.AllocIPAddresses(eni.EniId)
	if err != nil {
		return "", nil, errors.Wrapf(err, "Failed to alloc private ip on eni %s", eni.EniId)
	}
	ctxLog.Info("Successfully alloc private ip", "ip", ip, "eni id", eni.EniId)

	return ip, eni, nil
}

func (c *ServiceController) tryAssignPrivateIP(ctxLog logr.Logger, ip string, node agent.Node) (*pb.ENIMetadata, error) {
	ctxLog = ctxLog.WithName("tryAssignPrivateIP").WithValues("ip", ip)

	eni, err := node.ChooseENI()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to choose busiest ENI")
	}

	if err := c.cp.AssignPrivateIPAddresses(eni.EniId, ip); err != nil {
		return nil, errors.Wrapf(err, "Failed to assign private ip %s on eni %s", ip, eni.EniId)
	}
	ctxLog.Info("Successfully assign private ip")
	return eni, nil
}

func (c *ServiceController) chooseTrafficNode(svc *corev1.Service) (string, error) {
	ctxLog := c.logger.WithValues("svc", getObjectFullName(svc))
	var (
		annotations = svc.GetAnnotations()
	)
	masterNodeIP := annotations[AnnotationKeyMasterNodeIP]
	if masterNodeIP != "" {
		return masterNodeIP, nil
	}

	trafficPolicy, ok := annotations[AnnotationKeyTrafficPolicy]
	if !ok {
		trafficPolicy = DefaultTrafficPolicy
	}

	policy, ok := c.tps[trafficPolicy]
	if !ok {
		return "", fmt.Errorf("unknown traffic policy %s", trafficPolicy)
	}
	ctxLog.Info("Choosing traffic node", "policy", trafficPolicy)

	return policy.ChooseNode(svc)
}

func (c *ServiceController) buildFIPFromAnnotation(svc *corev1.Service) *FloatingIP {
	annotations := svc.GetAnnotations()
	result := &FloatingIP{
		ip:       annotations[AnnotationKeyFloatingIP],
		subnetID: annotations[AnnotationKeySubnetID],
		nodeIP:   annotations[AnnotationKeyENINodeIP],
		eni: &pb.ENIMetadata{
			EniId: annotations[AnnotationKeyENIId],
		},
	}
	return result
}
