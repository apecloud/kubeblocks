/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package controllerutil

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// ResultToP converts a Result object to a pointer.
func ResultToP(res reconcile.Result, err error) (*reconcile.Result, error) {
	return &res, err
}

// Reconciled returns an empty result with nil error to signal a successful reconcile
// to the controller manager
func Reconciled() (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

// CheckedRequeueWithError passes the error through to the controller
// manager, it ignores unknown errors.
func CheckedRequeueWithError(err error, logger logr.Logger, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	if apierrors.IsNotFound(err) {
		return Reconciled()
	}
	return RequeueWithError(err, logger, msg, keysAndValues...)
}

// RequeueWithErrorAndRecordEvent requeues when an error occurs. if it is an unknown error, triggers an event
func RequeueWithErrorAndRecordEvent(obj client.Object, recorder record.EventRecorder, err error, logger logr.Logger) (reconcile.Result, error) {
	if apierrors.IsNotFound(err) && recorder != nil {
		recorder.Eventf(obj, corev1.EventTypeWarning, constant.ReasonNotFoundCR, err.Error())
	}
	return RequeueWithError(err, logger, "")
}

// RequeueWithError requeues when an error occurs
func RequeueWithError(err error, logger logr.Logger, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	if msg == "" {
		logger.Info(err.Error())
	} else {
		// Info log the error message and then let the reconciler dump the stacktrace
		logger.Info(msg, keysAndValues...)
	}
	return reconcile.Result{}, err
}

func RequeueAfter(duration time.Duration, logger logr.Logger, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	keysAndValues = append(keysAndValues, "duration")
	keysAndValues = append(keysAndValues, duration)
	if msg != "" {
		msg = fmt.Sprintf("reason: %s; retry-after", msg)
	} else {
		msg = "retry-after"
	}
	logger.V(1).Info(msg, keysAndValues...)
	return reconcile.Result{
		Requeue:      true,
		RequeueAfter: duration,
	}, nil
}

func Requeue(logger logr.Logger, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	if msg == "" {
		msg = "requeue"
	}
	logger.V(1).Info(msg, keysAndValues...)
	return reconcile.Result{Requeue: true}, nil
}

// HandleCRDeletion handles CR deletion, adds finalizer if found a non-deleting object and removes finalizer during
// deletion process. Passes optional 'deletionHandler' func for external dependency deletion. Returns Result pointer
// if required to return out of outer 'Reconcile' reconciliation loop.
func HandleCRDeletion(reqCtx RequestCtx,
	r client.Writer,
	cr client.Object,
	finalizer string,
	deletionHandler func() (*ctrl.Result, error)) (*ctrl.Result, error) {
	// examine DeletionTimestamp to determine if object is under deletion
	if cr.GetDeletionTimestamp().IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then add the finalizer and update the object. This is equivalent to
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(cr, finalizer) {
			controllerutil.AddFinalizer(cr, finalizer)
			if err := r.Update(reqCtx.Ctx, cr); err != nil {
				return ResultToP(CheckedRequeueWithError(err, reqCtx.Log, ""))
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(cr, finalizer) {
			// We need to record the deletion event first.
			// If the resource has dependencies, it will not be automatically deleted.
			// It can also prevent users from manually deleting it without event records
			if reqCtx.Recorder != nil {
				cluster, ok := cr.(*v1alpha1.Cluster)
				// throw warning event if terminationPolicy set to DoNotTerminate
				if ok && cluster.Spec.TerminationPolicy == v1alpha1.DoNotTerminate {
					reqCtx.Eventf(cr, corev1.EventTypeWarning, constant.ReasonDeleteFailed,
						"Deleting %s: %s failed due to terminationPolicy set to DoNotTerminate",
						strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), cr.GetName())
				} else {
					reqCtx.Eventf(cr, corev1.EventTypeNormal, constant.ReasonDeletingCR, "Deleting %s: %s",
						strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), cr.GetName())
				}
			}

			// our finalizer is present, so handle any external dependency
			if deletionHandler != nil {
				if res, err := deletionHandler(); err != nil {
					// if failed to delete the external dependencies here, return with error
					// so that it can be retried
					if res == nil {
						return ResultToP(CheckedRequeueWithError(err, reqCtx.Log, ""))
					}
					return res, err
				} else if res != nil {
					return res, nil
				}
			}
			// remove our finalizer from the list and update it.
			if controllerutil.RemoveFinalizer(cr, finalizer) {
				if err := r.Update(reqCtx.Ctx, cr); err != nil {
					return ResultToP(CheckedRequeueWithError(err, reqCtx.Log, ""))
				}
				// record resources deleted event
				reqCtx.Eventf(cr, corev1.EventTypeNormal, constant.ReasonDeletedCR, "Deleted %s: %s",
					strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), cr.GetName())
			}
		}

		// Stop reconciliation as the item is being deleted
		res, err := Reconciled()
		return &res, err
	}
	return nil, nil
}

// ValidateReferenceCR validates existing referencing CRs, if exists, requeue reconcile after 30 seconds
func ValidateReferenceCR(reqCtx RequestCtx, cli client.Client, obj client.Object,
	labelKey string, recordEvent func(), objLists ...client.ObjectList) (*ctrl.Result, error) {
	for _, objList := range objLists {
		// get referencing cr list
		if err := cli.List(reqCtx.Ctx, objList,
			client.MatchingLabels{labelKey: obj.GetName()}, client.Limit(1),
		); err != nil {
			return nil, err
		}
		if v, err := conversion.EnforcePtr(objList); err != nil {
			return nil, err
		} else {
			// check list items
			items := v.FieldByName("Items")
			if !items.IsValid() || items.Kind() != reflect.Slice || items.Len() == 0 {
				continue
			}
			if recordEvent != nil {
				recordEvent()
			}
			return ResultToP(RequeueAfter(time.Second, reqCtx.Log, ""))
		}
	}
	return nil, nil
}

// RecordCreatedEvent records an event when a CR created successfully
func RecordCreatedEvent(r record.EventRecorder, cr client.Object) {
	if r != nil && cr.GetGeneration() == 1 {
		r.Eventf(cr, corev1.EventTypeNormal, constant.ReasonCreatedCR, "Created %s: %s", strings.ToLower(cr.GetObjectKind().GroupVersionKind().Kind), cr.GetName())
	}
}

// WorkloadFilterPredicate provides filter predicate for workload objects, i.e., deployment/statefulset/pod/pvc.
func WorkloadFilterPredicate(object client.Object) bool {
	_, containCompNameLabelKey := object.GetLabels()[constant.KBAppComponentLabelKey]
	return ManagedByKubeBlocksFilterPredicate(object) && containCompNameLabelKey
}

// ManagedByKubeBlocksFilterPredicate provides filter predicate for objects managed by kubeBlocks.
func ManagedByKubeBlocksFilterPredicate(object client.Object) bool {
	return object.GetLabels()[constant.AppManagedByLabelKey] == constant.AppName
}

// IgnoreIsAlreadyExists returns errors if 'err' is not type of AlreadyExists
func IgnoreIsAlreadyExists(err error) error {
	if !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// BackgroundDeleteObject deletes the object in the background, usually used in the Reconcile method
func BackgroundDeleteObject(cli client.Client, ctx context.Context, obj client.Object) error {
	deletePropagation := metav1.DeletePropagationBackground
	deleteOptions := &client.DeleteOptions{
		PropagationPolicy: &deletePropagation,
	}

	if err := cli.Delete(ctx, obj, deleteOptions); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

// SetOwnership provides helper function controllerutil.SetControllerReference/controllerutil.SetOwnerReference
// and controllerutil.AddFinalizer if not exists.
func SetOwnership(owner, obj client.Object, scheme *runtime.Scheme, finalizer string, useOwnerReference ...bool) error {
	if len(useOwnerReference) > 0 && useOwnerReference[0] {
		if err := controllerutil.SetOwnerReference(owner, obj, scheme); err != nil {
			return err
		}
	} else {
		if err := controllerutil.SetControllerReference(owner, obj, scheme); err != nil {
			return err
		}
	}
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		// pvc objects do not need to add finalizer
		_, ok := obj.(*corev1.PersistentVolumeClaim)
		_, isPod := obj.(*corev1.Pod)
		if !ok && !isPod {
			if !controllerutil.AddFinalizer(obj, finalizer) {
				return ErrFailedToAddFinalizer
			}
		}
	}
	return nil
}

// CheckResourceExists checks whether resource exist or not.
func CheckResourceExists(
	ctx context.Context,
	cli client.Client,
	key client.ObjectKey,
	obj client.Object) (bool, error) {
	if err := cli.Get(ctx, key, obj); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	// if found, return true
	return true, nil
}

var (
	portManager *PortManager
)

const (
	// https://www.w3.org/Daemon/User/Installation/PrivilegedPorts.html
	// The TCP/IP port numbers below 1024 are special in that normal users are not allowed to run servers on them.
	// This is a security feaure, in that if you connect to a service on one of these ports you can be fairly sure
	// that you have the real thing, and not a fake which some hacker has put up for you.
	hostPortMin = int32(1025)
	hostPortMax = int32(65536)
)

func InitHostPortManager(cli client.Client) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      viper.GetString(constant.CfgHostPortConfigMapName),
			Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
		},
		Data: make(map[string]string),
	}
	var err error
	if err = cli.Create(context.Background(), cm); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	portManager, err = NewPortManager(hostPortMin, hostPortMax, cli)
	return err
}

func GetPortManager() *PortManager {
	return portManager
}

func BuildHostPortName(clusterName, compName, containerName, portName string) string {
	return fmt.Sprintf("%s-%s-%s-%s", clusterName, compName, containerName, portName)
}

type PortManager struct {
	sync.Mutex
	cli  client.Client
	min  int32
	max  int32
	used map[int32]string
	cm   *corev1.ConfigMap
}

// NewPortManager creates a new PortManager
// TODO[ziang] Putting all the port information in one configmap may have performance issues and is not secure enough.
// There is a risk of accidental deletion leading to the loss of cluster port information.
func NewPortManager(min, max int32, cli client.Client) (*PortManager, error) {
	pm := &PortManager{
		min:  min,
		max:  max,
		cli:  cli,
		used: make(map[int32]string),
	}
	if err := pm.sync(); err != nil {
		return nil, err
	}
	return pm, nil
}

func (pm *PortManager) parsePort(port string) (int32, error) {
	port = strings.TrimSpace(port)
	if port == "" {
		return 0, fmt.Errorf("port is empty")
	}
	p, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(p), nil
}

func (pm *PortManager) sync() error {
	cm := &corev1.ConfigMap{}
	objKey := types.NamespacedName{
		Name:      viper.GetString(constant.CfgHostPortConfigMapName),
		Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
	}
	if err := pm.cli.Get(context.Background(), objKey, cm); err != nil {
		return err
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	used := make(map[int32]string)
	for key, item := range cm.Data {
		port, err := pm.parsePort(item)
		if err != nil {
			continue
		}
		used[port] = key
	}

	pm.cm = cm
	pm.used = used
	return nil
}

func (pm *PortManager) update(key string, port int32) error {
	var err error
	defer func() {
		if apierrors.IsConflict(err) {
			_ = pm.sync()
		}
	}()
	cm := pm.cm.DeepCopy()
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[key] = fmt.Sprintf("%d", port)
	err = pm.cli.Update(context.Background(), cm)
	if err != nil {
		return err
	}

	pm.cm = cm
	pm.used[port] = key
	return nil
}

func (pm *PortManager) delete(keys []string) error {
	if pm.cm.Data == nil {
		return nil
	}

	var err error
	defer func() {
		if apierrors.IsConflict(err) {
			_ = pm.sync()
		}
	}()

	cm := pm.cm.DeepCopy()
	var ports []int32
	for _, key := range keys {
		value, ok := cm.Data[key]
		if !ok {
			continue
		}
		port, err := pm.parsePort(value)
		if err != nil {
			return err
		}
		ports = append(ports, port)
		delete(cm.Data, key)
	}
	err = pm.cli.Update(context.Background(), cm)
	if err != nil {
		return err
	}
	pm.cm = cm
	for _, port := range ports {
		delete(pm.used, port)
	}
	return nil
}

func (pm *PortManager) UsePort(key string, port int32) error {
	pm.Lock()
	defer pm.Unlock()
	if k, ok := pm.used[port]; ok && k != key {
		return fmt.Errorf("port %d is used by %s", port, k)
	}
	if err := pm.update(key, port); err != nil {
		return err
	}
	return nil
}

func (pm *PortManager) AllocatePort(key string) (int32, error) {
	pm.Lock()
	defer pm.Unlock()

	if value, ok := pm.cm.Data[key]; ok {
		port, err := pm.parsePort(value)
		if err != nil {
			return 0, err
		}
		return port, nil
	}

	// allocate a new port randomly in range [hostPortMin, hostPortMax)
	for i := 0; i < 10; i++ {
		port := int32(rand.Int63nRange(int64(hostPortMin), int64(hostPortMax)))
		if _, ok := pm.used[port]; ok {
			continue
		}
		if err := pm.update(key, port); err != nil {
			return 0, err
		}
		return port, nil
	}
	return 0, fmt.Errorf("failed to allocate port")
}

func (pm *PortManager) ReleasePort(key string) error {
	return pm.ReleasePorts([]string{key})
}

func (pm *PortManager) ReleasePorts(keys []string) error {
	pm.Lock()
	defer pm.Unlock()
	for _, key := range keys {
		if err := pm.delete([]string{key}); err != nil {
			return err
		}
	}
	return nil
}

func (pm *PortManager) ReleaseByPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	pm.Lock()
	defer pm.Unlock()

	var keys []string
	for key := range pm.cm.Data {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	if err := pm.delete(keys); err != nil {
		return err
	}
	return nil
}

func (pm *PortManager) NeedAllocate(port int32) bool {
	return port <= 100
}
