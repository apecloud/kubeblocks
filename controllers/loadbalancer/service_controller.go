package loadbalancer

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	pb "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"
)

const (
	FinalizerKey = "service.kubernetes.io/apecloud-loadbalancer-finalizer"

	AnnotationKeyENIId        = "service.kubernetes.io/apecloud-loadbalancer-eni-id"
	AnnotationKeyENINodeIP    = "service.kubernetes.io/apecloud-loadbalancer-eni-node-ip"
	AnnotationKeyFloatingIP   = "service.kubernetes.io/apecloud-loadbalancer-floating-ip"
	AnnotationKeySubnetId     = "service.kubernetes.io/apecloud-loadbalancer-subnet-id"
	AnnotationKeyMasterNodeIP = "service.kubernetes.io/apecloud-loadbalancer-master-node-ip"

	AnnotationKeyLoadBalancerType   = "service.kubernetes.io/apecloud-loadbalancer-type"
	AnnotationValueLoadBalancerType = "private-ip"

	AnnotationKeyTrafficPolicy          = "service.kubernetes.io/apecloud-loadbalancer-traffic-policy"
	AnnotationValueClusterTrafficPolicy = "Cluster"
	AnnotationValueLocalTrafficPolicy   = "Local"
	DefaultTrafficPolicy                = AnnotationValueClusterTrafficPolicy
)

var (
	rpcPort        int64
	defaultRPCPort int64 = 19200
)

type FloatingIP struct {
	ip       string
	subnetId string
	nodeIP   string
	eni      *pb.ENIMetadata
}

type ServiceController struct {
	sync.RWMutex
	client.Client
	Scheme      *runtime.Scheme
	Recorder    record.EventRecorder
	logger      logr.Logger
	cp          cloud.Provider
	nc          pb.NodeCache
	subnetToENI map[string]map[string]bool
	cache       map[string]*FloatingIP
}

func init() {
	rpcPort = defaultRPCPort
	value := os.Getenv("RPC_PORT")
	if value != "" {
		port, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			panic(err)
		}
		rpcPort = port
	}
}

func NewServiceController(logger logr.Logger, client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder, cp cloud.Provider) (*ServiceController, error) {
	c := &ServiceController{
		Client:      client,
		Scheme:      scheme,
		Recorder:    recorder,
		logger:      logger,
		cp:          cp,
		nc:          pb.NewNodeCache(rpcPort),
		subnetToENI: make(map[string]map[string]bool),
		cache:       make(map[string]*FloatingIP),
	}

	nodeList := &corev1.NodeList{}
	if err := c.Client.List(context.Background(), nodeList); err != nil {
		return nil, errors.Wrap(err, "Failed to list cluster nodes")
	}
	if err := c.initNodes(nodeList); err != nil {
		return nil, errors.Wrap(err, "Failed to init nodes")
	}
	return c, nil
}

func (c *ServiceController) getFloatingIP(ip string) *FloatingIP {
	c.RLock()
	defer c.RUnlock()
	fip := c.cache[ip]
	if fip != nil {
		c.logger.Info("Get floating ip from cache", "ip", fip, "eni id", fip.eni.EniId)
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
		c.logger.Info("Delete floating ip from cache", "ip", ip, "eni id", fip.eni.EniId)
	}
}

func (c *ServiceController) initNodes(nodeList *corev1.NodeList) error {
	for _, item := range nodeList.Items {
		var nodeIP string
		for _, addr := range item.Status.Addresses {
			if addr.Type != corev1.NodeInternalIP {
				continue
			}
			nodeIP = addr.Address
		}
		if nodeIP == "" {
			c.logger.Error(fmt.Errorf("invalid cluster node %v", item), "Skip init node")
			continue
		}
		if err := c.initNode(nodeIP); err != nil {
			return errors.Wrapf(err, "Failed to init node %s", nodeIP)
		}
	}
	return nil
}

func (c *ServiceController) initNode(nodeIP string) error {
	ctxLog := c.logger.WithValues("node", nodeIP)

	node, err := c.nc.GetNode(nodeIP)
	if err != nil {
		return errors.Wrapf(err, "Failed to get rpc client for node %s", nodeIP)
	}
	resp, err := node.GetManagedENIs(context.Background(), &pb.GetManagedENIsRequest{RequestId: getRequestId()})
	if err != nil {
		return errors.Wrapf(err, "Failed to query enis from node %s", nodeIP)
	}
	for _, eni := range resp.GetEnis() {

		nodeIPs, ok := c.subnetToENI[eni.SubnetId]
		if !ok {
			nodeIPs = make(map[string]bool)
		}
		nodeIPs[nodeIP] = true
		c.subnetToENI[eni.SubnetId] = nodeIPs

		for _, addr := range eni.Ipv4Addresses {
			request := &pb.SetupNetworkForServiceRequest{
				RequestId: getRequestId(),
				PrivateIp: addr.Address,
				Eni:       eni,
			}
			if _, err := node.SetupNetworkForService(context.Background(), request); err != nil {
				return errors.Wrapf(err, "Failed to init service private ip %s for node %s", addr.Address, nodeIP)
			}
			c.setFloatingIP(addr.Address, &FloatingIP{ip: addr.Address, subnetId: eni.SubnetId, nodeIP: nodeIP, eni: eni})
			ctxLog.Info("Successfully init service", "private ip", addr.Address)
		}
	}
	return nil
}

func (c *ServiceController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&corev1.Service{}).Complete(c)
}

func (c *ServiceController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxLog := c.logger.WithValues("service", req.NamespacedName.String())
	ctxLog.Info("Receive service reconcile event")

	svc := &corev1.Service{}
	if err := c.Client.Get(ctx, req.NamespacedName, svc); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, c.logger, "")
	}

	annotations := svc.GetAnnotations()
	if _, ok := annotations[AnnotationKeyLoadBalancerType]; !ok {
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
	c.logger.Info("Migrating floating ip", "src eni", fip.eni.EniId, "ip", fip.ip)

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
	ctxLog.WithName("migrateOnNewMaster").WithValues("node", nodeIP)

	if err := c.cp.DeallocIPAddresses(fip.eni.EniId, []string{fip.ip}); err != nil {
		return errors.Wrapf(err, "Failed to dealloc private ip %s", fip.ip)
	}

	node, err := c.nc.GetNode(nodeIP)
	if err != nil {
		return errors.Wrap(err, "Failed to get master node")
	}

	newENI, err := c.tryAssignPrivateIP(ctxLog, fip.ip, node)
	if err != nil {
		return errors.Wrap(err, "Failed to assign private ip")
	}
	newFip := &FloatingIP{ip: fip.ip, subnetId: fip.subnetId, nodeIP: nodeIP, eni: newENI}
	c.setFloatingIP(newFip.ip, newFip)

	setupServiceRequest := &pb.SetupNetworkForServiceRequest{
		RequestId: getRequestId(),
		PrivateIp: newFip.ip,
		Eni:       newENI,
	}

	if _, err = node.SetupNetworkForService(ctx, setupServiceRequest); err != nil {
		return errors.Wrap(err, "Failed to setup host network stack for service")
	}

	if err = c.updateService(ctx, ctxLog, svc, newFip, false); err != nil {
		return err
	}
	return nil
}

func (c *ServiceController) migrateOnOldMaster(ctx context.Context, ctxLog logr.Logger, fip *FloatingIP) error {
	ctxLog = ctxLog.WithName("migrateOnOldMaster").WithValues("node", fip.nodeIP)

	node, err := c.nc.GetNode(fip.nodeIP)
	if err != nil {
		return errors.Wrap(err, "Failed to get old master node")
	}
	cleanServiceRequest := &pb.CleanNetworkForServiceRequest{
		RequestId: getRequestId(),
		PrivateIp: fip.ip,
		Eni:       fip.eni,
	}
	if _, err := node.CleanNetworkForService(ctx, cleanServiceRequest); err != nil {
		return errors.Wrap(err, "Failed to cleanup private ip")
	}
	ctxLog.Info("Successfully clean service network ")
	return nil
}

func (c *ServiceController) createFloatingIP(ctx context.Context, ctxLog logr.Logger, nodeIP string, svc *corev1.Service) error {
	ctxLog = ctxLog.WithName("createFloatingIP").WithValues("node", nodeIP)

	node, err := c.nc.GetNode(nodeIP)
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
		subnetId: eni.SubnetId,
		nodeIP:   nodeIP,
		eni:      eni,
	}
	c.setFloatingIP(privateIP, fip)

	request := &pb.SetupNetworkForServiceRequest{
		RequestId: getRequestId(),
		PrivateIp: fip.ip,
		Eni:       fip.eni,
	}
	if _, err = node.SetupNetworkForService(ctx, request); err != nil {
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
		c.logger.Info("Service floating ip is nil, skip delete")
		return nil
	}
	ctxLog = ctxLog.WithName("deleteFloatingIP").WithValues("node", fip.nodeIP)

	annotations := svc.GetAnnotations()
	eniId, ok := annotations[AnnotationKeyENIId]
	if !ok {
		return errors.New("Invalid service, private ip exists but eni id not found")
	}

	c.logger.Info("Deleting service private ip", "eni id", eniId, "ip", fip.ip)
	if err := c.cp.DeallocIPAddresses(eniId, []string{fip.ip}); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to dealloc private ip address %s", fip.ip))
	}
	ctxLog.Info("Successfully released floating ip")

	node, err := c.nc.GetNode(fip.nodeIP)
	if err != nil {
		return errors.Wrapf(err, "Failed to get rpc client for node %s", fip.nodeIP)
	}
	request := &pb.CleanNetworkForServiceRequest{
		RequestId: getRequestId(),
		PrivateIp: fip.ip,
		Eni:       fip.eni,
	}
	if _, err = node.CleanNetworkForService(ctx, request); err != nil {
		return errors.Wrap(err, "Failed to cleanup private ip")
	}
	ctxLog.Info("Successfully clean service network on node")

	c.removeFloatingIP(fip.ip)
	ctxLog.Info("Successfully remove floating ip from cache")

	if err := c.updateService(ctx, ctxLog, svc, fip, true); err != nil {
		return err
	}
	ctxLog.Info("Successfully deleted floating ip and cleaned it's host networking")
	return nil

}

func (c *ServiceController) updateService(ctx context.Context, logger logr.Logger, svc *corev1.Service, fip *FloatingIP, deleting bool) error {
	annotations := svc.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[AnnotationKeyENIId] = fip.eni.EniId
	annotations[AnnotationKeyENINodeIP] = fip.nodeIP
	annotations[AnnotationKeyFloatingIP] = fip.ip
	annotations[AnnotationKeySubnetId] = fip.subnetId
	svc.SetAnnotations(annotations)

	if deleting {
		controllerutil.RemoveFinalizer(svc, FinalizerKey)
	} else {
		controllerutil.AddFinalizer(svc, FinalizerKey)
	}

	svc.Spec.ExternalIPs = []string{fip.ip}

	svcName := fmt.Sprintf("%s/%s", svc.GetNamespace(), svc.GetName())
	if err := c.Client.Update(ctx, svc); err != nil {
		return errors.Wrapf(err, "Failed to update service %s", svcName)
	}
	logger.Info("Successfully update service", "info", svc.String())
	return nil
}

func (c *ServiceController) tryAllocPrivateIP(ctxLog logr.Logger, node pb.NodeClient) (string, *pb.ENIMetadata, error) {
	ctxLog = ctxLog.WithName("tryAllocPrivateIP")

	request := &pb.ChooseBusiestENIRequest{
		RequestId: getRequestId(),
	}
	resp, err := node.ChooseBusiestENI(context.Background(), request)
	if err != nil {
		return "", nil, errors.Wrap(err, "Failed to choose busiest ENI")
	}
	eni := resp.Eni
	ctxLog.Info("Successfully choose busiest eni", "eni id", eni.EniId)

	ip, err := c.cp.AllocIPAddresses(eni.EniId)
	if err != nil {
		return "", nil, errors.Wrap(err, fmt.Sprintf("Failed to alloc private ip on eni %s", eni.EniId))
	}
	ctxLog.Info("Successfully alloc private ip", "ip", ip, "eni id", eni.EniId)

	return ip, eni, nil
}

func (c *ServiceController) tryAssignPrivateIP(ctxLog logr.Logger, ip string, node pb.NodeClient) (*pb.ENIMetadata, error) {
	ctxLog = ctxLog.WithName("tryAssignPrivateIP").WithValues("ip", ip)

	request := &pb.ChooseBusiestENIRequest{
		RequestId: getRequestId(),
	}
	resp, err := node.ChooseBusiestENI(context.Background(), request)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to choose busiest ENI")
	}

	eni := resp.Eni
	if err := c.cp.AssignPrivateIpAddresses(eni.EniId, ip); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Failed to assign private ip %s on eni %s", ip, eni.EniId))
	}
	ctxLog.Info("Successfully assign private ip")
	return eni, nil
}

func (c *ServiceController) chooseTrafficNode(svc *corev1.Service) (string, error) {
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
	if trafficPolicy == AnnotationValueClusterTrafficPolicy {
		nodeIP, ok := annotations[AnnotationKeyENINodeIP]
		if ok {
			return nodeIP, nil
		}
		// TODO
	}

	matchLabels := client.MatchingLabels{}
	for k, v := range svc.Spec.Selector {
		matchLabels[k] = v
	}
	listOptions := []client.ListOption{
		matchLabels,
		client.InNamespace(svc.GetNamespace()),
	}
	pods := &corev1.PodList{}
	if err := c.Client.List(context.Background(), pods, listOptions...); err != nil {
		return "", errors.Wrap(err, "Failed to list service related pods")
	}
	if len(pods.Items) == 0 {
		return "", errors.New(fmt.Sprintf("Can not find master node for service %s", getServiceFullName(svc)))
	}

	c.logger.Info("Found master pods", "count", len(pods.Items))
	return pods.Items[0].Status.HostIP, nil
}

func getServiceFullName(service *corev1.Service) string {
	return fmt.Sprintf("%s/%s", service.GetNamespace(), service.GetName())
}

func getRequestId() string {
	return uuid.New().String()
}
