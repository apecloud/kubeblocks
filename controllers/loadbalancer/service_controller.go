package loadbalancer

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/grpc/credentials/insecure"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	pb "github.com/apecloud/kubeblocks/internal/loadbalancer/protocol"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
)

const (
	FinalizerKey                    = "service.kubernetes.io/apecloud-loadbalancer-finalizer"
	AnnotationKeyENIId              = "service.kubernetes.io/apecloud-loadbalancer-eni-id"
	AnnotationKeyENINodeIP          = "service.kubernetes.io/apecloud-loadbalancer-eni-node-ip"
	AnnotationKeyFloatingIP         = "service.kubernetes.io/apecloud-loadbalancer-floating-ip"
	AnnotationKeyMasterNodeIP       = "service.kubernetes.io/apecloud-loadbalancer-master-node-ip"
	AnnotationKeyLoadBalancerType   = "service.kubernetes.io/apecloud-loadbalancer-type"
	AnnotationValueLoadBalancerType = "private-ip"

	RoleNewMaster = "new_master"
	RoleOldMaster = "old_master"
	RoleOthers    = "others"
)

var (
	rpcPort        int64
	defaultRPCPort int64 = 19200
)

type FloatingIP struct {
	ip     string
	nodeIP string
	eni    *pb.ENIMetadata
}

type ServiceController struct {
	sync.RWMutex
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	logger   logr.Logger
	cp       cloud.Provider
	nodes    map[string]pb.NodeClient
	cache    map[string]*FloatingIP
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
		Client:   client,
		Scheme:   scheme,
		Recorder: recorder,
		logger:   logger,
		cp:       cp,
		nodes:    make(map[string]pb.NodeClient),
		cache:    make(map[string]*FloatingIP),
	}
	if err := c.initNodes(); err != nil {
		return nil, errors.Wrap(err, "Failed to init nodes")
	}
	return c, nil
}

func (c *ServiceController) initNodes() error {
	clusterNodes := &corev1.NodeList{}
	if err := c.Client.List(context.Background(), clusterNodes); err != nil {
		return errors.Wrap(err, "Failed to list cluster nodes")
	}

	for _, item := range clusterNodes.Items {
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

	node, err := c.getNode(nodeIP)
	if err != nil {
		return errors.Wrapf(err, "Failed to get rpc client for node %s", nodeIP)
	}
	resp, err := node.GetManagedENIs(context.Background(), &pb.GetManagedENIsRequest{RequestId: uuid.New().String()})
	if err != nil {
		return errors.Wrapf(err, "Failed to query enis from node %s", nodeIP)
	}
	for _, eni := range resp.GetEnis() {
		for _, addr := range eni.Ipv4Addresses {
			request := &pb.SetupNetworkForServiceRequest{
				RequestId: uuid.New().String(),
				PrivateIp: addr.Address,
				Eni:       eni,
			}
			if _, err := node.SetupNetworkForService(context.Background(), request); err != nil {
				return errors.Wrapf(err, "Failed to init service private ip %s for node %s", addr.Address, nodeIP)
			}
			c.SetFloatingIP(addr.Address, &FloatingIP{ip: addr.Address, nodeIP: nodeIP, eni: eni})
			ctxLog.Info("Successfully init service", "private ip", addr.Address)
		}
	}
	return nil
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

	if err := c.migrateFloatingIP(ctx, ctxLog, svc); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, ctxLog, "")
	}

	return intctrlutil.Reconciled()
}

func (c *ServiceController) migrateFloatingIP(ctx context.Context, ctxLog logr.Logger, svc *corev1.Service) error {
	var (
		err         error
		annotations = svc.GetAnnotations()
		fip         = c.GetFloatingIP(annotations[AnnotationKeyFloatingIP])
	)
	// service is in deleting
	if !svc.GetDeletionTimestamp().IsZero() {
		return c.deleteFloatingIP(ctx, ctxLog, fip, svc)
	}

	masterNodeIP, err := c.getMasterNodeIP(svc)
	if err != nil {
		return errors.Wrap(err, "Failed to get master nodes")
	}

	// service is in creating, just created
	if fip == nil {
		return c.createFloatingIP(ctx, ctxLog, masterNodeIP, svc)
	}

	c.logger.Info("Migrating floating ip", "src eni", fip.eni.EniId, "ip", fip.ip)
	var (
		wg   = sync.WaitGroup{}
		errs []error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs = append(errs, c.migrateOnNewMaster(ctx, ctxLog, masterNodeIP, fip, svc))
	}()
	go func() {
		defer wg.Done()
		errs = append(errs, c.migrateOnOldMaster(ctx, ctxLog, fip))
	}()
	wg.Wait()

	var messages []string
	for _, err = range errs {
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

func (c *ServiceController) migrateOnNewMaster(ctx context.Context, ctxLog logr.Logger, masterNodeIP string, fip *FloatingIP, svc *corev1.Service) error {
	ctxLog.WithName("migrateOnNewMaster").WithValues("node", masterNodeIP)

	if err := c.cp.DeallocIPAddresses(fip.eni.EniId, []string{fip.ip}); err != nil {
		return errors.Wrapf(err, "Failed to dealloc private ip %s", fip.ip)
	}

	node, err := c.getNode(masterNodeIP)
	if err != nil {
		return errors.Wrap(err, "Failed to get master node")
	}

	chooseENIRequest := &pb.ChooseBusiestENIRequest{
		RequestId: uuid.New().String(),
	}
	resp, err := node.ChooseBusiestENI(ctx, chooseENIRequest)
	if err != nil {
		return errors.Wrap(err, "Failed to choose busiest ENI")
	}

	if err = c.cp.AssignPrivateIpAddresses(resp.Eni.EniId, fip.ip); err != nil {
		return errors.Wrapf(err, "Failed to assign private ip %s on eni %s", fip.ip, fip.eni.EniId)
	}
	fip = &FloatingIP{ip: fip.ip, nodeIP: masterNodeIP, eni: resp.Eni}
	c.SetFloatingIP(fip.ip, fip)

	setupServiceRequest := &pb.SetupNetworkForServiceRequest{
		RequestId: uuid.New().String(),
		PrivateIp: fip.ip,
		Eni:       resp.Eni,
	}

	if _, err = node.SetupNetworkForService(ctx, setupServiceRequest); err != nil {
		return errors.Wrap(err, "Failed to setup host network stack for service")
	}

	if err := c.updateService(ctx, ctxLog, svc, fip, false); err != nil {
		return err
	}
	return nil
}

func (c *ServiceController) migrateOnOldMaster(ctx context.Context, ctxLog logr.Logger, fip *FloatingIP) error {
	ctxLog = ctxLog.WithName("migrateOnOldMaster").WithValues("node", fip.nodeIP)

	node, err := c.getNode(fip.nodeIP)
	if err != nil {
		return errors.Wrap(err, "Failed to get old master node")
	}
	cleanServiceRequest := &pb.CleanNetworkForServiceRequest{
		RequestId: uuid.New().String(),
		PrivateIp: fip.ip,
		Eni:       fip.eni,
	}
	if _, err := node.CleanNetworkForService(ctx, cleanServiceRequest); err != nil {
		return errors.Wrap(err, "Failed to cleanup private ip")
	}
	ctxLog.Info("Successfully clean service network ")
	return nil
}

func (c *ServiceController) createFloatingIP(ctx context.Context, ctxLog logr.Logger, masterNodeIP string, svc *corev1.Service) error {
	ctxLog = ctxLog.WithName("createFloatingIP").WithValues("node", masterNodeIP)

	masterNode, err := c.getNode(masterNodeIP)
	if err != nil {
		return errors.Wrap(err, "Failed to get master node")
	}

	privateIP, eni, err := c.tryAllocPrivateIP(ctxLog, masterNode)
	if err != nil {
		return errors.Wrap(err, "Failed to alloc new private ip for service")
	}
	ctxLog.Info("Successfully alloc private ip")

	fip := &FloatingIP{
		ip:     privateIP,
		nodeIP: masterNodeIP,
		eni:    eni,
	}
	c.SetFloatingIP(privateIP, fip)

	request := &pb.SetupNetworkForServiceRequest{
		RequestId: uuid.New().String(),
		PrivateIp: fip.ip,
		Eni:       fip.eni,
	}
	if _, err = masterNode.SetupNetworkForService(ctx, request); err != nil {
		return errors.Wrap(err, "Failed to setup host network stack for service")
	}
	ctxLog.Info("Successfully setup service host network")

	if err = c.updateService(ctx, ctxLog, svc, fip, false); err != nil {
		return errors.Wrap(err, "Failed to update service")
	}

	return nil
}

func (c *ServiceController) deleteFloatingIP(ctx context.Context, ctxLog logr.Logger, fip *FloatingIP, svc *corev1.Service) error {
	ctxLog = ctxLog.WithName("deleteFloatingIP").WithValues("node", fip.nodeIP)

	if fip == nil {
		c.logger.Info("Service floating ip is nil, skip delete")
		return nil
	}

	if err := c.deletePrivateIP(ctx, fip.ip, svc); err != nil {
		return errors.Wrap(err, "Failed to delete private ip")
	}
	ctxLog.Info("Successfully released floating ip")

	node, err := c.getNode(fip.nodeIP)
	if err != nil {
		return errors.Wrapf(err, "Failed to get rpc client for node %s", fip.nodeIP)
	}
	request := &pb.CleanNetworkForServiceRequest{
		RequestId: uuid.New().String(),
		PrivateIp: fip.ip,
		Eni:       fip.eni,
	}
	if _, err = node.CleanNetworkForService(ctx, request); err != nil {
		return errors.Wrap(err, "Failed to cleanup private ip")
	}
	ctxLog.Info("Successfully clean service network on node")

	c.DeleteFloatingIP(fip.ip)
	ctxLog.Info("Successfully remove floating ip from cache")

	if err := c.updateService(ctx, ctxLog, svc, fip, true); err != nil {
		return err
	}
	ctxLog.Info("Successfully deleted floating ip and cleaned it's host networking")
	return nil

}

func (c *ServiceController) GetFloatingIP(ip string) *FloatingIP {
	c.RLock()
	defer c.RUnlock()
	fip := c.cache[ip]
	if fip != nil {
		c.logger.Info("Get ip from cache", "ip", fip, "eni id", fip.eni.EniId)
	}
	return fip
}

func (c *ServiceController) SetFloatingIP(ip string, fip *FloatingIP) {
	c.Lock()
	defer c.Unlock()
	c.cache[ip] = fip
	c.logger.Info("Put ip to cache", "ip", ip, "eni id", fip.eni.EniId)
}

func (c *ServiceController) DeleteFloatingIP(ip string) {
	c.Lock()
	defer c.Unlock()
	fip := c.cache[ip]
	delete(c.cache, ip)
	if fip != nil {
		c.logger.Info("Delete ip from cache", "ip", ip, "eni id", fip.eni.EniId)
	}
}

func (c *ServiceController) deletePrivateIP(ctx context.Context, privateIP string, svc *corev1.Service) error {
	annotations := svc.GetAnnotations()
	eniId, ok := annotations[AnnotationKeyENIId]
	if !ok {
		return errors.New("Invalid service, private ip exists but eni id not found")
	}

	c.logger.Info("Deleting service private ip", "eni id", eniId, "ip", privateIP)
	if err := c.cp.DeallocIPAddresses(eniId, []string{privateIP}); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to dealloc private ip address %s", privateIP))
	}
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
		RequestId: uuid.New().String(),
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
		RequestId: uuid.New().String(),
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

func (c *ServiceController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&corev1.Service{}).Complete(c)
}

/*
func (c *ServiceController) restorePolicyRulesForServices(enis []cp.ENIMetadata) error {
	rules, err := c.networkClient.GetRuleList()
	if err != nil {
		return errors.Wrap(err, "Failed to retrieve IP rule list")
	}

	for _, eni := range enis {
		ipList, err := c.cp.GetIPv4sFromEC2(eni.ENIId)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Failed to get private ips on eni %s", eni.ENIId))
		}

		for _, ip := range ipList {
			// Update ip rules in case there is a change in VPC CIDRs, AWS_VPC_K8S_CNI_EXTERNALSNAT setting
			srcIPNet := net.IPNet{IP: net.ParseIP(aws.StringValue(ip.PrivateIpAddress)), Mask: net.CIDRMask(32, 32)}

			err = c.networkClient.UpdateRuleListBySrc(rules, srcIPNet)
			if err != nil {
				c.logger.Error(err, "UpdateRuleListBySrc in nodeInit failed", "private IP", aws.StringValue(ip.PrivateIpAddress))
			}
		}
	}
	return nil
}
*/

func (c *ServiceController) getMasterNodeIP(svc *corev1.Service) (string, error) {
	masterNodeIP := svc.GetAnnotations()[AnnotationKeyMasterNodeIP]
	if masterNodeIP != "" {
		return masterNodeIP, nil
	}

	matchLabels := client.MatchingLabels{}
	for k, v := range svc.Spec.Selector {
		matchLabels[k] = v
	}
	listOptions := []client.ListOption{
		client.InNamespace(svc.GetNamespace()),
		matchLabels,
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

func (c *ServiceController) getMasterNode(svc *corev1.Service) (pb.NodeClient, error) {
	ip, err := c.getMasterNodeIP(svc)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get master node ip")
	}
	return c.getNode(ip)
}

func (c *ServiceController) getNode(ip string) (pb.NodeClient, error) {
	var node pb.NodeClient

	c.RLock()
	node, ok := c.nodes[ip]
	c.RUnlock()
	if ok {
		return node, nil
	}

	addr := fmt.Sprintf("%s:%d", ip, rpcPort)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to dial: %v", addr)
	}
	node = pb.NewNodeClient(conn)
	c.Lock()
	c.nodes[ip] = node
	c.Unlock()
	return node, nil
}

func getServiceFullName(service *corev1.Service) string {
	return fmt.Sprintf("%s/%s", service.GetNamespace(), service.GetName())
}
