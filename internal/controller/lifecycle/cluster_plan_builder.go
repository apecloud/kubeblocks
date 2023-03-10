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

package lifecycle

import (
	"errors"
	"fmt"
	"golang.org/x/exp/maps"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type clusterPlanBuilder struct {
	ctx     intctrlutil.RequestCtx
	cli     client.Client
	cluster *appsv1alpha1.Cluster
}

type clusterPlan struct {
	dag      *graph.DAG
	walkFunc graph.WalkFunc
}

func (b *clusterPlanBuilder) getCompoundCluster() (*compoundCluster, error) {
	cd := &appsv1alpha1.ClusterDefinition{}
	if err := b.cli.Get(b.ctx.Ctx, types.NamespacedName{
		Name: b.cluster.Spec.ClusterDefRef,
	}, cd); err != nil {
		return nil, err
	}
	cv := &appsv1alpha1.ClusterVersion{}
	if len(b.cluster.Spec.ClusterVersionRef) > 0 {
		if err := b.cli.Get(b.ctx.Ctx, types.NamespacedName{
			Name: b.cluster.Spec.ClusterVersionRef,
		}, cv); err != nil {
			return nil, err
		}
	}

	cc := &compoundCluster{
		cluster: b.cluster,
		cd:      *cd,
		cv:      *cv,
	}
	return cc, nil
}

func (b *clusterPlanBuilder) defaultWalkFunc(node graph.Vertex) error {
	obj, ok := node.(*lifecycleVertex)
	if !ok {
		return fmt.Errorf("wrong node type %v", node)
	}
	if obj.action == nil {
		return errors.New("node action can't be nil")
	}
	switch *obj.action {
	case CREATE:
		err := b.cli.Create(b.ctx.Ctx, obj.obj)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	case UPDATE:
		if obj.immutable {
			return nil
		}
		o, err := buildUpdateObj(obj)
		if err != nil {
			return err
		}
		err = b.cli.Update(b.ctx.Ctx, o)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	case DELETE:
		err := b.cli.Delete(b.ctx.Ctx, obj.obj)
		if err != nil && apierrors.IsNotFound(err) {
			return err
		}
	case STATUS:
		patch := client.MergeFrom(obj.oriObj)
		return b.cli.Status().Patch(b.ctx.Ctx, obj.obj, patch)
	}
	return nil
}

func buildUpdateObj(node *lifecycleVertex) (client.Object, error) {
	handleSts := func(origObj, stsProto *appsv1.StatefulSet) (client.Object, error) {
		//if *stsObj.Spec.Replicas != *stsProto.Spec.Replicas {
		//	reqCtx.Recorder.Eventf(cluster,
		//		corev1.EventTypeNormal,
		//		"HorizontalScale",
		//		"Start horizontal scale component %s from %d to %d",
		//		component.Name,
		//		*stsObj.Spec.Replicas,
		//		*stsProto.Spec.Replicas)
		//}
		stsObj := origObj.DeepCopy()
		// keep the original template annotations.
		// if annotations exist and are replaced, the statefulSet will be updated.
		stsProto.Spec.Template.Annotations = mergeAnnotations(stsObj.Spec.Template.Annotations,
			stsProto.Spec.Template.Annotations)
		stsObj.Spec.Template = stsProto.Spec.Template
		stsObj.Spec.Replicas = stsProto.Spec.Replicas
		stsObj.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
		if !reflect.DeepEqual(&origObj.Spec, &stsObj.Spec) {
			// sync component phase
			// TODO: syncComponentPhaseWhenSpecUpdating
			//syncComponentPhaseWhenSpecUpdating(cluster, componentName)
		}

		return stsObj, nil
	}

	handleDeploy := func(origObj, deployProto *appsv1.Deployment) (client.Object, error) {
		deployObj := origObj.DeepCopy()
		deployProto.Spec.Template.Annotations = mergeAnnotations(deployObj.Spec.Template.Annotations,
			deployProto.Spec.Template.Annotations)
		deployObj.Spec = deployProto.Spec
		if !reflect.DeepEqual(&origObj.Spec, &deployObj.Spec) {
			// sync component phase
			// TODO: syncComponentPhaseWhenSpecUpdating
			//componentName := deployObj.Labels[constant.KBAppComponentLabelKey]
			//syncComponentPhaseWhenSpecUpdating(cluster, componentName)
		}
		return deployObj, nil
	}

	handleSvc := func(origObj, svcProto *corev1.Service) (client.Object, error) {
		svcObj := origObj.DeepCopy()
		svcObj.Spec = svcProto.Spec
		svcObj.Annotations = mergeServiceAnnotations(svcObj.Annotations, svcProto.Annotations)
		return svcObj, nil
	}

	handlePVC := func(origObj, pvcProto *corev1.PersistentVolumeClaim) (client.Object, error) {
		pvcObj := origObj.DeepCopy()
		if pvcObj.Spec.Resources.Requests[corev1.ResourceStorage] == pvcProto.Spec.Resources.Requests[corev1.ResourceStorage] {
			return pvcObj, nil
		}
		pvcObj.Spec.Resources.Requests[corev1.ResourceStorage] = pvcProto.Spec.Resources.Requests[corev1.ResourceStorage]
		return pvcObj, nil
	}

	switch node.obj.(type) {
	case *appsv1.StatefulSet:
		return handleSts(node.oriObj.(*appsv1.StatefulSet), node.obj.(*appsv1.StatefulSet))
	case *appsv1.Deployment:
		return handleDeploy(node.oriObj.(*appsv1.Deployment), node.obj.(*appsv1.Deployment))
	case *corev1.Service:
		return handleSvc(node.oriObj.(*corev1.Service), node.obj.(*corev1.Service))
	case *corev1.PersistentVolumeClaim:
		return handlePVC(node.oriObj.(*corev1.PersistentVolumeClaim), node.obj.(*corev1.PersistentVolumeClaim))
	case *corev1.Secret, *corev1.ConfigMap:
		return node.obj, nil
	}

	return nil, nil
}

// Build only cluster Creation, Update and Deletion supported.
// TODO: Validations and Corrections (cluster labels correction, primaryIndex spec validation etc.)
func (b *clusterPlanBuilder) Build() (graph.Plan, error) {
	cc, err := b.getCompoundCluster()
	if err != nil {
		return nil, err
	}

	// build transformer chain
	chain := &graph.TransformerChain{
		// cluster to K8s objects and put them into dag
		&clusterTransformer{cc: *cc, cli: b.cli, ctx: b.ctx},
		// tls certs secret
		&tlsCertsTransformer{cc: *cc, cli: b.cli, ctx: b.ctx},
		// add our finalizer to all objects
		&ownershipTransformer{finalizer: dbClusterFinalizerName},
		// make all workload objects depending on credential secret
		&credentialTransformer{},
		// make config configmap immutable
		&configTransformer{},
		// read old snapshot from cache, and generate diff plan
		&cacheDiffTransformer{cc: *cc, cli: b.cli, ctx: b.ctx},
		// horizontal scaling
		&horizontalScalingTransformer{cc: *cc, cli: b.cli, ctx: b.ctx},
		// stateful set pvc Update
		&statefulSetPVCTransformer{cc: *cc, cli: b.cli, ctx: b.ctx},
		// finally, update cluster status
		&clusterStatusTransformer{*cc},
	}

	// new a DAG and apply chain on it, after that we should get the final Plan
	dag := graph.NewDAG()
	if err := chain.ApplyTo(dag); err != nil {
		return nil, err
	}

	// we got the execution plan
	plan := &clusterPlan{
		dag:      dag,
		walkFunc: b.defaultWalkFunc,
	}
	return plan, nil
}

// NewClusterPlanBuilder returns a clusterPlanBuilder powered PlanBuilder
// TODO: change ctx to context.Context
func NewClusterPlanBuilder(ctx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster) graph.PlanBuilder {
	return &clusterPlanBuilder{
		ctx:     ctx,
		cli:     cli,
		cluster: cluster,
	}
}

func (p *clusterPlan) Execute() error {
	return p.dag.WalkReverseTopoOrder(p.walkFunc)
}

// mergeAnnotations keeps the original annotations.
// if annotations exist and are replaced, the Deployment/StatefulSet will be updated.
func mergeAnnotations(originalAnnotations, targetAnnotations map[string]string) map[string]string {
	if restartAnnotation, ok := originalAnnotations[constant.RestartAnnotationKey]; ok {
		if targetAnnotations == nil {
			targetAnnotations = map[string]string{}
		}
		targetAnnotations[constant.RestartAnnotationKey] = restartAnnotation
	}
	return targetAnnotations
}

// mergeServiceAnnotations keeps the original annotations except prometheus scrape annotations.
// if annotations exist and are replaced, the Service will be updated.
func mergeServiceAnnotations(originalAnnotations, targetAnnotations map[string]string) map[string]string {
	if len(originalAnnotations) == 0 {
		return targetAnnotations
	}
	tmpAnnotations := make(map[string]string, len(originalAnnotations)+len(targetAnnotations))
	for k, v := range originalAnnotations {
		if !strings.HasPrefix(k, "prometheus.io") {
			tmpAnnotations[k] = v
		}
	}
	maps.Copy(tmpAnnotations, targetAnnotations)
	return tmpAnnotations
}
