package loadbalancer

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/agent"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/cloud"
	"github.com/apecloud/kubeblocks/internal/loadbalancer/network"
)

const (
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

	EnvHostIP = "HOST_IP"
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
	logger   logr.Logger
	em       agent.ENIManager
	nc       network.Client
	cp       cloud.Provider
	hostIP   string
	cache    map[string]*cloud.ENIMetadata
}

func NewServiceController(logger logr.Logger, client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder, em agent.ENIManager, cp cloud.Provider, nc network.Client) (*ServiceController, error) {
	c := &ServiceController{
		Client:   client,
		Scheme:   scheme,
		Recorder: recorder,
		cp:       cp,
		nc:       nc,
		em:       em,
		logger:   logger,
		cache:    make(map[string]*cloud.ENIMetadata),
	}

	hostIPStr, found := os.LookupEnv(EnvHostIP)
	if !found {
		return nil, errors.New("Failed to init host IP")
	} else {
		c.hostIP = hostIPStr
	}
	c.logger.Info("Init host local ip", "ip", c.hostIP)

	enis, err := em.GetManagedENIs()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get managed enis")
	}
	return c, c.initPrivateIPs(enis)
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

func (c *ServiceController) initPrivateIPs(enis []*cloud.ENIMetadata) error {
	for i := range enis {
		eni := enis[i]

		for _, privateIP := range eni.IPv4Addresses {
			ip := aws.StringValue(privateIP.PrivateIpAddress)
			if err := c.nc.SetupNetworkForService(ip, eni); err != nil {
				return errors.Wrapf(err, "Failed to init service, private ip: %s", ip)
			}
			c.SetPrivateIP(ip, eni)
			c.logger.Info("Successfully init service", "private ip", ip)
		}
	}
	return nil
}

func (c *ServiceController) newMasterReconcile(ctx context.Context, ctxLog logr.Logger, svc *corev1.Service) error {
	var (
		annotations = svc.GetObjectMeta().GetAnnotations()
		privateIP   = annotations[AnnotationKeyPrivateIP]
		deleting    = !svc.GetDeletionTimestamp().IsZero()
		cachedENI   = c.GetPrivateIP(privateIP)
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
			c.DeletePrivateIP(privateIP)
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
		cachedENI   = c.GetPrivateIP(privateIP)
	)

	if err := c.nc.CleanNetworkForService(privateIP, cachedENI); err != nil {
		return errors.Wrap(err, "Failed to cleanup private ip")
	}
	c.DeletePrivateIP(privateIP)
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
		if ok && c.GetPrivateIP(privateIP) != nil {
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

func (c *ServiceController) GetPrivateIP(privateIP string) *cloud.ENIMetadata {
	c.RLock()
	defer c.RUnlock()
	eni := c.cache[privateIP]
	if eni != nil {
		c.logger.Info("Get private ip from cache", "private ip", privateIP, "eni id", eni.ENIId)
	}
	return eni
}

func (c *ServiceController) SetPrivateIP(privateIP string, eni *cloud.ENIMetadata) {
	c.Lock()
	defer c.Unlock()
	c.cache[privateIP] = eni
	c.logger.Info("Put private ip to cache", "private ip", privateIP, "eni id", eni.ENIId)
}

func (c *ServiceController) DeletePrivateIP(privateIP string) {
	c.Lock()
	defer c.Unlock()
	eni := c.cache[privateIP]
	delete(c.cache, privateIP)
	if eni != nil {
		c.logger.Info("Delete private ip from cache", "private ip", privateIP, "eni id", eni.ENIId)
	}
}

func (c *ServiceController) createPrivateIP(ctx context.Context, svc *corev1.Service) (string, error) {
	privateIP, eni, err := c.tryAllocPrivateIP()
	if err != nil {
		return "", errors.Wrap(err, "Failed to alloc new private ip for service")
	}
	c.SetPrivateIP(privateIP, eni)

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
	if err := c.cp.DeallocIPAddresses(eniId, []string{privateIP}); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to dealloc private ip address %s", privateIP))
	}

	eni, err := c.tryAssignPrivateIP(privateIP)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to assign ip address %s", privateIP))
	}
	c.SetPrivateIP(privateIP, eni)

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
	if err := c.cp.DeallocIPAddresses(eniId, []string{privateIP}); err != nil {
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
	eni, err := c.em.ChooseBusiestENI()
	if err != nil {
		return "", nil, errors.Wrap(err, "Failed to choose busiest ENI")
	}

	res, err := c.cp.AllocIPAddresses(eni.ENIId)
	if err != nil {
		return "", nil, errors.Wrap(err, fmt.Sprintf("Failed to alloc private ip address on eni %s", eni.ENIId))
	}

	ip := aws.StringValue(res.AssignedPrivateIpAddresses[0].PrivateIpAddress)
	c.logger.Info("Successfully alloc service private ip", "ip", ip, "eni id", eni.ENIId)
	return ip, eni, nil
}

func (c *ServiceController) tryAssignPrivateIP(ip string) (*cloud.ENIMetadata, error) {
	eni, err := c.em.ChooseBusiestENI()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to choose busiest ENI")
	}

	if err := c.cp.AssignPrivateIpAddresses(eni.ENIId, ip); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("Failed to assign private ip address %s on eni %s", ip, eni.ENIId))
	}
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
