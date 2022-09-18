package loadbalancer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apecloud/kubeblocks/internal/dbctl/util"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	iptableswrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/iptables"
	netlinkwrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/netlink"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/network"
	procfswrapper "github.com/apecloud/kubeblocks/internal/loadbalancer/procfs"
)

const (
	EnvMaxENI     = "MAX_ENI"
	DefaultMaxENI = -1

	EnvMinPrivateIP     = "MIN_PRIVATE_IP"
	DefaultMinPrivateIP = 1

	EnvHostIP = "HOST_IP"

	FinalizerKey                    = "service.kubernetes.io/apecloud-loadbalancer-finalizer"
	AnnotationKeyENIId              = "service.kubernetes.io/apecloud-loadbalancer-eni-id"
	AnnotationKeyENIHost            = "service.kubernetes.io/apecloud-loadbalancer-eni-host"
	AnnotationKeyPrivateIP          = "service.kubernetes.io/apecloud-loadbalancer-private-ip"
	AnnotationKeyMasterHost         = "service.kubernetes.io/apecloud-loadbalancer-master-host"
	AnnotationKeyLoadBalancerType   = "service.kubernetes.io/apecloud-loadbalancer-type"
	AnnotationValueLoadBalancerType = "private-ip"

	RoleNewMaster = "new_master"
	RoleOldMaster = "old_master"
	RoleOthers    = "others"
)

// TODO assign multiple private ip from different subnets
// TODO if mask of src address in policy rule little than 32 bit, causing routing problem
// TODO include iptables binary in agent image
// TODO monitor security group / subnet / vpc changes

type ServiceController struct {
	sync.RWMutex
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	logger       logr.Logger
	nc           network.Client
	cloud        cloud.Service
	hostIP       string
	maxIPsPerENI int
	maxENI       int
	minPrivateIP int
	cachedENIs   map[string]*cloud.ENIMetadata
}

func NewServiceController(logger logr.Logger, client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder) (*ServiceController, error) {
	c := &ServiceController{
		logger:     logger,
		Client:     client,
		Scheme:     scheme,
		Recorder:   recorder,
		cachedENIs: make(map[string]*cloud.ENIMetadata),
	}

	awsSvc, err := cloud.NewService("aws", logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize AWS SDK cloud")
	}
	c.cloud = awsSvc

	ipStr, found := os.LookupEnv(EnvMinPrivateIP)
	envMin := DefaultMinPrivateIP
	if found {
		if input, err := strconv.Atoi(ipStr); err == nil && input >= 1 {
			c.logger.Info("Using MIN_PRIVATE_IP", "count", input)
			envMin = input
		}
	}
	c.minPrivateIP = envMin

	hostIPStr, found := os.LookupEnv(EnvHostIP)
	if !found {
		return nil, errors.New("Failed to init host IP")
	} else {
		c.hostIP = hostIPStr
	}
	c.logger.Info("Init host local ip", "ip", c.hostIP)

	ipt, err := iptableswrapper.NewIPTables()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to init iptables")
	}
	nc, err := network.NewClient(c.logger, netlinkwrapper.NewNetLink(), ipt, procfswrapper.NewProcFS())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to init network client")
	}
	c.nc = nc

	if err := c.initENIAndIPLimits(); err != nil {
		return nil, errors.Wrap(err, "Failed to get eni and private ip limits")
	}

	return c, c.initNode()
}

func (c *ServiceController) StartENIController() error {
	enis, err := c.cloud.DescribeAllENIs()
	if err != nil {
		return errors.Wrap(err, "Failed to get all enis, retry later")
	}

	var primaryENI *cloud.ENIMetadata
	for _, eni := range enis {
		if eni.DeviceNumber == 0 {
			primaryENI = eni
			break
		}
	}
	if primaryENI == nil {
		return errors.Wrap(err, "Failed to find primary eni")
	}
	// TODO enable check when we can ensure traffic comes in and out from same interface
	if err := c.cloud.ModifySourceDestCheck(primaryENI.ENIId, true); err != nil {
		return errors.Wrap(err, "Failed to disable primary eni source/destination check")
	}
	c.logger.Info("Successfully disable primary eni source/destination check")

	f := func() {
		if err := c.ensureENI(); err != nil {
			c.logger.Error(err, "Failed to ensure eni")
		}
	}
	go wait.Forever(f, 15*time.Second)

	return nil
}

func (c *ServiceController) ensureENI() error {
	enis, err := c.cloud.DescribeAllENIs()
	if err != nil {
		return errors.Wrap(err, "Failed to get all enis, retry later")
	}

	var (
		managedENIs = c.filterManagedENIs(enis)
		status      = make(map[string]interface{}, len(managedENIs))
		totalSpare  int
		min         = c.minPrivateIP
		max         = c.minPrivateIP + c.maxIPsPerENI
	)
	for _, eni := range managedENIs {
		spare := c.maxIPsPerENI - len(eni.IPv4Addresses)
		status[eni.ENIId] = map[string]int{
			"total": c.maxIPsPerENI,
			"used":  len(eni.IPv4Addresses),
			"spare": spare,
		}
		totalSpare += spare
	}

	c.logger.Info("Local private ip buffer status",
		"spare private ip", totalSpare, "min spare private ip", min, "max spare private ip", max)

	b, _ := json.Marshal(status)
	c.logger.Info("Local private ip buffer status", "info", string(b))

	if totalSpare < min {
		if err := c.tryAllocAndAttachENI(); err != nil {
			c.logger.Error(err, "Failed to alloc and attach new ENI")
		}
	} else if totalSpare > max {
		if err := c.tryDetachAndDeleteENI(managedENIs); err != nil {
			c.logger.Error(err, "Failed to detach and delete idle ENI")
		}
	}
	return nil
}

func (c *ServiceController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctxLog := c.logger.WithValues("service", req.NamespacedName.String())
	ctxLog.Info("Receive service reconcile event", "name", req.NamespacedName)

	svc := &corev1.Service{}
	if err := c.Client.Get(ctx, req.NamespacedName, svc); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, c.logger, "")
	}

	role, err := c.checkRoleForService(ctx, svc)
	if err != nil {
		ctxLog.Error(err, "Failed to check local role")
		return intctrlutil.CheckedRequeueWithError(err, c.logger, "")
	}
	if role == RoleOthers {
		ctxLog.Info("Ignore unrelated service")
		return intctrlutil.Reconciled()
	}

	privateIP := svc.GetObjectMeta().GetAnnotations()[AnnotationKeyPrivateIP]
	ctxLog = ctxLog.WithValues("private ip", privateIP)

	// creating / migrating / deleting service, on new master host
	if role == RoleNewMaster {
		if err := c.newMasterReconcile(ctx, ctxLog, svc); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, c.logger, "")
		}
	}

	// tidy up service network, on old master host
	if role == RoleOldMaster {
		if err := c.oldMasterReconcile(ctx, ctxLog, svc); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, c.logger, "")
		}
	}

	return intctrlutil.Reconciled()
}

func (c *ServiceController) newMasterReconcile(ctx context.Context, ctxLog logr.Logger, svc *corev1.Service) error {
	var (
		annotations = svc.GetObjectMeta().GetAnnotations()
		privateIP   = annotations[AnnotationKeyPrivateIP]
		deleting    = !svc.GetDeletionTimestamp().IsZero()
		cachedENI   = c.getCachedENI(privateIP)
	)

	// creating service
	if privateIP == "" {
		privateIP, err := c.createPrivateIP(ctx, svc)
		if err != nil {
			return errors.Wrap(err, "Failed to create private ip")
		}
		ctxLog.Info("Successfully create private ip", "info", privateIP)
		return nil
	}

	// deleting service
	if deleting {
		if err := c.deletePrivateIP(ctx, privateIP, svc); err != nil {
			return errors.Wrap(err, "Failed to delete private ip")
		}
		if cachedENI != nil {
			if err := c.nc.CleanNetworkForService(privateIP, cachedENI); err != nil {
				return errors.Wrap(err, "Failed to cleanup private ip")
			}
			c.deleteCachedENI(privateIP)
		} else {
			ctxLog.Info("Can not find cached private ip, skip clean network for service")
		}
		ctxLog.Info("Successfully unassigned private ip and cleaned it's host networking")
		return nil
	}

	// migrating service
	if cachedENI != nil {
		ctxLog.Info("Found private ip at local cache, skip migrate")
		return nil
	}
	if err := c.migratePrivateIP(ctx, privateIP, svc); err != nil {
		return errors.Wrap(err, "Failed to migrate private ip")
	}
	ctxLog.Info("Successfully migrate private ip to local")
	return nil
}

func (c *ServiceController) oldMasterReconcile(ctx context.Context, ctxLog logr.Logger, svc *corev1.Service) error {
	var (
		annotations = svc.GetObjectMeta().GetAnnotations()
		privateIP   = annotations[AnnotationKeyPrivateIP]
		cachedENI   = c.getCachedENI(privateIP)
	)

	if err := c.nc.CleanNetworkForService(privateIP, cachedENI); err != nil {
		return errors.Wrap(err, "Failed to cleanup private ip")
	}
	c.deleteCachedENI(privateIP)
	ctxLog.Info("Successfully cleaned host networking for private ip")
	return nil
}

func (c *ServiceController) checkRoleForService(ctx context.Context, svc *corev1.Service) (string, error) {
	var (
		role        string
		reason      string
		annotations = svc.GetObjectMeta().GetAnnotations()
	)

	ctxLog := c.logger.WithValues("service", fmt.Sprintf("%s/%s", svc.Namespace, svc.Name))

	if _, ok := annotations[AnnotationKeyLoadBalancerType]; !ok {
		return RoleOthers, nil
	}

	if hostIP, ok := annotations[AnnotationKeyMasterHost]; ok {
		if hostIP == c.hostIP {
			role = RoleNewMaster
			reason = "Found master annotation at local"
		}
	} else {
		listOptions := []client.ListOption{client.InNamespace(svc.GetNamespace())}
		matchLabels := client.MatchingLabels{}
		for k, v := range svc.Spec.Selector {
			matchLabels[k] = v
		}
		listOptions = append(listOptions, matchLabels)
		pods := &corev1.PodList{}
		if err := c.Client.List(ctx, pods, listOptions...); err != nil {
			return RoleOthers, errors.Wrap(err, "Failed to list service related pods")
		}
		if len(pods.Items) > 0 {
			ctxLog.Info("Found master pods", "count", len(pods.Items))
			pod := pods.Items[0]
			if pod.Status.HostIP == c.hostIP {
				role = RoleNewMaster
				reason = "Found rw pod on local host"
			} else {
				ctxLog.Info("Found master pod at other host", "pod", pod.Status.HostIP)
			}
		}
	}

	if role == "" {
		privateIP, ok := annotations[AnnotationKeyPrivateIP]
		if ok && c.getCachedENI(privateIP) != nil {
			role = RoleOldMaster
			reason = "Found cached private ip"
		} else {
			role = RoleOthers
		}
	}
	if reason != "" {
		ctxLog.Info("Check local role", "reason", reason, "role", role)
	}
	return role, nil
}

func (c *ServiceController) getCachedENI(privateIP string) *cloud.ENIMetadata {
	c.RLock()
	defer c.RUnlock()
	eni := c.cachedENIs[privateIP]
	if eni != nil {
		c.logger.Info("Get private ip from cache", "private ip", privateIP, "eni id", eni.ENIId)
	}
	return eni
}

func (c *ServiceController) putCachedENI(privateIP string, eni *cloud.ENIMetadata) {
	c.Lock()
	defer c.Unlock()
	c.cachedENIs[privateIP] = eni
	c.logger.Info("Put private ip to cache", "private ip", privateIP, "eni id", eni.ENIId)
}

func (c *ServiceController) deleteCachedENI(privateIP string) {
	c.Lock()
	defer c.Unlock()
	eni := c.cachedENIs[privateIP]
	delete(c.cachedENIs, privateIP)
	if eni != nil {
		c.logger.Info("Delete private ip from cache", "private ip", privateIP, "eni id", eni.ENIId)
	}
}

func (c *ServiceController) getCachedENIs() map[string]bool {
	c.RLock()
	defer c.RUnlock()
	result := make(map[string]bool, len(c.cachedENIs))
	for _, eni := range c.cachedENIs {
		if _, ok := result[eni.ENIId]; !ok {
			result[eni.ENIId] = true
		}
	}
	return result
}

func (c *ServiceController) createPrivateIP(ctx context.Context, svc *corev1.Service) (string, error) {
	privateIP, eni, err := c.tryAllocPrivateIP()
	if err != nil {
		return "", errors.Wrap(err, "Failed to alloc new private ip for service")
	}
	c.putCachedENI(privateIP, eni)

	if err := c.nc.SetupNetworkForService(privateIP, eni); err != nil {
		return "", errors.Wrap(err, "Failed to configure policy rules for service")
	}

	if err := c.updateService(ctx, svc, eni.ENIId, privateIP, false); err != nil {
		return "", errors.Wrap(err, "Failed to update service annotation")
	}

	return privateIP, nil
}

func (c *ServiceController) migratePrivateIP(ctx context.Context, privateIP string, svc *corev1.Service) error {
	annotations := svc.GetAnnotations()
	eniId, ok := annotations[AnnotationKeyENIId]
	if !ok {
		return errors.New("Invalid service, private ip exists but eni id not found")
	}

	c.logger.Info("Migrating service private ip", "src eni", eniId, "ip", privateIP)
	if err := c.cloud.DeallocIPAddresses(eniId, []string{privateIP}); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to dealloc private ip address %s", privateIP))
	}

	eni, err := c.tryAssignPrivateIP(privateIP)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to assign ip address %s", privateIP))
	}
	c.putCachedENI(privateIP, eni)

	if err := c.nc.SetupNetworkForService(privateIP, eni); err != nil {
		return errors.Wrap(err, "Failed to configure policy rules for service")
	}

	if err := c.updateService(ctx, svc, eni.ENIId, privateIP, false); err != nil {
		return err
	}
	return nil
}

func (c *ServiceController) deletePrivateIP(ctx context.Context, privateIP string, svc *corev1.Service) error {
	annotations := svc.GetAnnotations()
	eniId, ok := annotations[AnnotationKeyENIId]
	if !ok {
		return errors.New("Invalid service, private ip exists but eni id not found")
	}

	c.logger.Info("Deleting service private ip", "eni id", eniId, "ip", privateIP)
	if err := c.cloud.DeallocIPAddresses(eniId, []string{privateIP}); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to dealloc private ip address %s", privateIP))
	}

	if err := c.updateService(ctx, svc, eniId, privateIP, true); err != nil {
		return err
	}
	return nil
}

func (c *ServiceController) updateService(ctx context.Context, svc *corev1.Service, eniId string, privateIP string, deleting bool) error {
	annotations := svc.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[AnnotationKeyENIId] = eniId
	annotations[AnnotationKeyENIHost] = c.hostIP
	annotations[AnnotationKeyPrivateIP] = privateIP
	svc.SetAnnotations(annotations)

	if deleting {
		controllerutil.RemoveFinalizer(svc, FinalizerKey)
	} else {
		controllerutil.AddFinalizer(svc, FinalizerKey)
	}

	svc.Spec.ExternalIPs = []string{privateIP}

	svcName := fmt.Sprintf("%s/%s", svc.GetNamespace(), svc.GetName())
	if err := c.Client.Update(ctx, svc); err != nil {
		return errors.Wrapf(err, "Failed to update service %s", svcName)
	}
	c.logger.Info("Successfully update service", "info", svc.String())
	return nil
}

func (c *ServiceController) tryAllocPrivateIP() (string, *cloud.ENIMetadata, error) {
	eni, err := c.chooseBusiestENI()
	if err != nil {
		return "", nil, errors.Wrap(err, "Failed to choose busiest ENI")
	}

	res, err := c.cloud.AllocIPAddresses(eni.ENIId)
	if err != nil {
		return "", nil, errors.Wrap(err, fmt.Sprintf("Failed to alloc private ip address on eni %s", eni.ENIId))
	}

	ip := aws.StringValue(res.AssignedPrivateIpAddresses[0].PrivateIpAddress)
	c.logger.Info("Successfully alloc service private ip", "ip", ip, "eni id", eni.ENIId)
	return ip, eni, nil
}

func (c *ServiceController) tryAssignPrivateIP(ip string) (*cloud.ENIMetadata, error) {
	eni, err := c.chooseBusiestENI()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to choose busiest ENI")
	}

	if err := c.cloud.AssignPrivateIpAddresses(eni.ENIId, ip); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Failed to assign private ip address %s on eni %s", ip, eni.ENIId))
	}
	return eni, nil
}

func (c *ServiceController) chooseBusiestENI() (*cloud.ENIMetadata, error) {
	enis, err := c.cloud.DescribeAllENIs()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get attached ENIs info")
	}

	managedENIs := c.filterManagedENIs(enis)
	if len(managedENIs) == 0 {
		return nil, errors.New("No managed eni found")
	}
	candidate := managedENIs[0]
	for _, eni := range managedENIs {
		if len(eni.IPv4Addresses) > len(candidate.IPv4Addresses) && len(eni.IPv4Addresses) < c.maxIPsPerENI {
			candidate = eni
		}
	}
	c.logger.Info("Found busiest eni", "eni id", candidate.ENIId)
	return candidate, nil
}

func (c *ServiceController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&corev1.Service{}).Complete(c)
}

func (c *ServiceController) initNode() error {
	enis, err := c.cloud.DescribeAllENIs()
	if err != nil {
		return errors.Wrap(err, "ipamd init: failed to retrieve attached ENIs info")
	}

	managedENIs := c.filterManagedENIs(enis)
	for i := range managedENIs {
		eni := managedENIs[i]
		c.logger.Info("Discovered managed ENI, trying to set it up", "eni id", eni.ENIId)

		options := &util.RetryOptions{MaxRetry: 3, Delay: 10 * time.Second}
		if err := util.DoWithRetry(context.Background(), c.logger, func() error {
			return c.nc.SetupNetworkForENI(eni)
		}, options); err != nil {
			c.logger.Error(err, "Failed to setup ENI", "eni id", eni.ENIId)
		} else {
			c.logger.Info("ENI set up completed", "eni id", eni.ENIId)
		}

		for _, privateIP := range eni.IPv4Addresses {
			ip := aws.StringValue(privateIP.PrivateIpAddress)
			if err := c.nc.SetupNetworkForService(ip, eni); err != nil {
				return errors.Wrapf(err, "Failed to init service, private ip: %s", ip)
			}
			c.putCachedENI(ip, eni)
			c.logger.Info("Successfully init private ip", "private ip", ip)
		}
	}
	c.logger.Info("Successfully init node")

	return nil
}

func (c *ServiceController) filterManagedENIs(enis map[string]*cloud.ENIMetadata) []*cloud.ENIMetadata {
	var (
		ids            []string
		managedENIList []*cloud.ENIMetadata
	)
	for eniId, eni := range enis {
		if _, found := eni.Tags[cloud.TagENIKubeBlocksManaged]; !found {
			continue
		}

		ids = append(ids, eniId)
		managedENIList = append(managedENIList, enis[eniId])
	}
	c.logger.Info("Managed eni", "count", len(managedENIList), "ids", strings.Join(ids, ","))
	return managedENIList
}

/*
func (c *ServiceController) restorePolicyRulesForServices(enis []cloud.ENIMetadata) error {
	rules, err := c.networkClient.GetRuleList()
	if err != nil {
		return errors.Wrap(err, "Failed to retrieve IP rule list")
	}

	for _, eni := range enis {
		ipList, err := c.cloud.GetIPv4sFromEC2(eni.ENIId)
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

func (c *ServiceController) initENIAndIPLimits() (err error) {
	nodeMaxENI, err := c.getMaxENI()
	if err != nil {
		c.logger.Error(err, "Failed to get ENI limit")
		return err
	}
	c.maxENI = nodeMaxENI

	c.maxIPsPerENI = c.cloud.GetENIIPv4Limit()
	if err != nil {
		return err
	}
	c.logger.Info("Query resource quota", "max eni", c.maxENI, "max private ip per eni", c.maxIPsPerENI)

	return nil
}

// getMaxENI returns the maximum number of ENIs to attach to this instance. This is calculated as the lesser of
// the limit for the instance type and the value configured via the MAX_ENI environment variable. If the value of
// the environment variable is 0 or less, it will be ignored and the maximum for the instance is returned.
func (c *ServiceController) getMaxENI() (int, error) {
	instanceMaxENI := c.cloud.GetENILimit()

	inputStr, found := os.LookupEnv(EnvMaxENI)
	envMax := DefaultMaxENI
	if found {
		if input, err := strconv.Atoi(inputStr); err == nil && input >= 1 {
			c.logger.Info("Using MAX_ENI", "count", input)
			envMax = input
		}
	}

	if envMax >= 1 && envMax < instanceMaxENI {
		return envMax, nil
	}
	return instanceMaxENI, nil
}

func (c *ServiceController) tryAllocAndAttachENI() error {
	c.logger.Info("Try to alloc and attach new eni")

	enis := c.getCachedENIs()
	if len(enis) >= c.maxENI {
		c.logger.Info("Limit exceed, can not alloc new eni", "current", enis, "max", c.maxENI)
		return nil
	}

	// alloc ENI, use same sg and subnet as primary ENI
	eniId, err := c.cloud.AllocENI()
	if err != nil {
		return errors.Wrap(err, "Failed to alloc ENI, retry later")
	}
	c.logger.Info("Successfully create new eni, waiting for attached", "eni id", eniId)

	// waiting for ENI attached
	eni, err := c.cloud.WaitForENIAndIPsAttached(eniId)
	if err != nil {
		return errors.Wrap(err, "Unable to discover attached ENI from metadata service")
	}
	c.logger.Info("New eni attached", "eni id", eniId)

	// setup ENI networking stack
	if err := c.nc.SetupNetworkForENI(&eni); err != nil {
		return errors.Wrapf(err, "Failed to set up network for eni %s", eni.ENIId)
	}
	c.logger.Info("Successfully initialized new eni", "eni id", eniId)
	return nil
}

func (c *ServiceController) tryDetachAndDeleteENI(enis []*cloud.ENIMetadata) error {
	c.logger.Info("Try to detach and delete idle eni")

	for _, eni := range enis {
		if len(eni.IPv4Addresses) > 1 {
			continue
		}
		if !aws.BoolValue(eni.IPv4Addresses[0].Primary) {
			continue
		}
		if err := c.nc.CleanNetworkForENI(eni); err != nil {
			return errors.Wrapf(err, "Failed to clean network for eni %s", eni.ENIId)
		}
		if err := c.cloud.FreeENI(eni.ENIId); err != nil {
			return errors.Wrapf(err, "Failed to free eni %s", eni.ENIId)
		}
		c.logger.Info("Successfully detach and delete idle eni", "eni id", eni.ENIId)
		return nil
	}
	return errors.New("Failed to find a idle eni")
}
