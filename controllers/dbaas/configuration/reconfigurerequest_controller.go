/*
Copyright ApeCloud Inc.

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

package configuration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/component"
	cfgpolicy "github.com/apecloud/kubeblocks/controllers/dbaas/configuration/policy"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/configmap"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ReconfigureRequestReconciler reconciles a ReconfigureRequest object
type ReconfigureRequestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

var ConfigurationRequiredLabels = []string{
	intctrlutil.AppNameLabelKey,
	intctrlutil.AppInstanceLabelKey,
	intctrlutil.AppComponentLabelKey,
	cfgcore.CMConfigurationTplNameLabelKey,
	cfgcore.CMConfigurationTplLabelKey,
}

//+kubebuilder:rbac:groups=core,resources=configmap,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmap/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=configmap/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ReconfigureRequest object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *ReconfigureRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("Configuration", req.NamespacedName),
		Recorder: r.Recorder,
	}

	config := &corev1.ConfigMap{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, config); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "not find configmap", "key", req.NamespacedName)
	}

	if !checkConfigurationObject(config) {
		return intctrlutil.Reconciled()
	}

	if hash, ok := config.Labels[cfgcore.CMInsConfigurationHashLabelKey]; ok && hash == config.ResourceVersion {
		return intctrlutil.Reconciled()
	}

	isAppliedCfg, err := ApplyConfigurationChange(r.Client, reqCtx, config)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to check last-applied-configuration")
	} else if isAppliedCfg {
		return intctrlutil.Reconciled()
	}

	if cfgConstraintsName, ok := config.Labels[cfgcore.CMConfigurationConstraintsNameLabelKey]; !ok || len(cfgConstraintsName) == 0 {
		reqCtx.Log.Info("configuration not set ConfigConstraints, not support reconfigure.", "config cm", client.ObjectKeyFromObject(config))
		return intctrlutil.Reconciled()
	}

	tpl := &dbaasv1alpha1.ConfigurationTemplate{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: config.Namespace,
		Name:      config.Labels[cfgcore.CMConfigurationConstraintsNameLabelKey],
	}, tpl); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config, r.Recorder, err, reqCtx.Log)
	}

	return r.sync(reqCtx, config, tpl)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReconfigureRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(predicate.NewPredicateFuncs(checkConfigurationObject)).
		Complete(r)
}

func checkConfigurationObject(object client.Object) bool {
	return CheckConfigurationLabels(object, ConfigurationRequiredLabels)
}

func (r *ReconfigureRequestReconciler) sync(reqCtx intctrlutil.RequestCtx, config *corev1.ConfigMap, tpl *dbaasv1alpha1.ConfigurationTemplate) (ctrl.Result, error) {

	var (
		stsLists   = appv1.StatefulSetList{}
		cluster    = dbaasv1alpha1.Cluster{}
		clusterKey = client.ObjectKey{
			Namespace: config.GetNamespace(),
			Name:      config.Labels[intctrlutil.AppInstanceLabelKey],
		}

		configTplName     = config.Labels[cfgcore.CMConfigurationISVTplLabelKey]
		configTplLabelKey = cfgcore.GenerateTPLUniqLabelKeyWithConfig(configTplName)
	)

	componentLabels := map[string]string{
		intctrlutil.AppNameLabelKey:      config.Labels[intctrlutil.AppNameLabelKey],
		intctrlutil.AppInstanceLabelKey:  config.Labels[intctrlutil.AppInstanceLabelKey],
		intctrlutil.AppComponentLabelKey: config.Labels[intctrlutil.AppComponentLabelKey],
		configTplLabelKey:                config.GetName(),
	}

	versionMeta, err := GetConfigurationVersion(config, reqCtx, &tpl.Spec)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config, r.Recorder, err, reqCtx.Log)
	}

	// Not any parameters updated
	if !versionMeta.IsModify {
		return r.updateCfgStatus(reqCtx, config, ReconfigureNoChangeType)
	}

	reqCtx.Log.Info(fmt.Sprintf("reconfigure params: \n\tadd: %s\n\tdelete: %s\n\tupdate: %s",
		versionMeta.AddConfig,
		versionMeta.DeleteConfig,
		versionMeta.UpdateConfig))

	// Find Cluster CR
	if err := r.Client.Get(reqCtx.Ctx, clusterKey, &cluster); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			cfgcore.WrapError(err,
				"failed to get cluster. name[%s]", clusterKey),
			reqCtx.Log)
	}

	// Find ClusterComponent from cluster cr
	componentName := config.Labels[intctrlutil.AppComponentLabelKey]
	clusterComponent := GetClusterComponentsByName(cluster.Spec.Components, componentName)
	// fix cluster maybe not any component
	if clusterComponent == nil {
		reqCtx.Log.Info("not found component.", "componentName", componentName,
			"clusterName", cluster.GetName())
	} else {
		componentName = clusterComponent.Type
	}

	// Find ClusterDefinition Component  from ClusterDefinition CR
	component, err := component.GetComponentFromClusterDefinition(reqCtx.Ctx, r.Client, &cluster, componentName)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			cfgcore.WrapError(err,
				"failed to get component from cluster definition. type[%s]", componentName),
			reqCtx.Log)
	} else if component == nil {
		logrus.Warnf("failed to found component which the configuration is associated, component name: %s", componentName)
		return intctrlutil.Reconciled()
	}

	if component.ConfigSpec == nil {
		return intctrlutil.Reconciled()
	}

	// find STS CR
	if err := r.Client.List(reqCtx.Ctx, &stsLists, client.InNamespace(config.Namespace), client.MatchingLabels(componentLabels)); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			cfgcore.WrapError(err,
				"failed to get component. configmap[%s] label[%s]",
				reqCtx.Req.NamespacedName, componentLabels),
			reqCtx.Log)
	}

	// configmap has never been used
	sts, containersList := GetComponentByUsingCM(&stsLists, client.ObjectKeyFromObject(config))
	if len(sts) == 0 {
		reqCtx.Log.Info("configmap is not used by any container.", "cm name", client.ObjectKeyFromObject(config))
		return intctrlutil.Reconciled()
	}

	return r.performUpgrade(cfgpolicy.ReconfigureParams{
		TplName:          configTplName,
		Meta:             versionMeta,
		Cfg:              config,
		Tpl:              &tpl.Spec,
		Client:           r.Client,
		Ctx:              reqCtx,
		Cluster:          &cluster,
		ContainerName:    containersList,
		ComponentUnits:   sts,
		Component:        component,
		ClusterComponent: clusterComponent,
		Restart:          cfgcm.IsNotSupportReload(component.ConfigSpec.ReconfigureOption),
	})
}

func (r *ReconfigureRequestReconciler) updateCfgStatus(reqCtx intctrlutil.RequestCtx, cfg *corev1.ConfigMap, reconfigureType string) (ctrl.Result, error) {
	configData, err := json.Marshal(cfg.Data)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(cfg, r.Recorder, err, reqCtx.Log)
	}

	if ok, err := UpdateAppliedConfiguration(r.Client, reqCtx, cfg, configData, reconfigureType); err != nil || !ok {
		return intctrlutil.RequeueAfter(ConfigReconcileInterval, reqCtx.Log, "failed to patch status and retry...", "error", err)
	}

	return intctrlutil.Reconciled()
}

func (r *ReconfigureRequestReconciler) performUpgrade(params cfgpolicy.ReconfigureParams) (ctrl.Result, error) {
	policy, err := cfgpolicy.NewReconfigurePolicy(params.Tpl, params.Meta, GetUpgradePolicy(params.Cfg), params.Restart)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(params.Cfg, r.Recorder, err, params.Ctx.Log)
	}

	execStatus, err := policy.Upgrade(params)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(params.Cfg, r.Recorder, err, params.Ctx.Log)
	}

	switch execStatus {
	case cfgpolicy.ESRetry:
		return intctrlutil.RequeueAfter(ConfigReconcileInterval, params.Ctx.Log, "")
	case cfgpolicy.ESNone:
		return r.updateCfgStatus(params.Ctx, params.Cfg, policy.GetPolicyName())
	case cfgpolicy.ESFailed:
		if err := SetCfgUpgradeFlag(params.Client, params.Ctx, params.Cfg, false); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, params.Ctx.Log, "")
		}
		return intctrlutil.Reconciled()
	default:
		return intctrlutil.Reconciled()
	}
}
