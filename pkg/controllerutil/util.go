/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"reflect"
	"strings"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/generics"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// GetUncachedObjects returns a list of K8s objects, for these object types,
// and their list types, client.Reader will read directly from the API server instead
// of the cache, which may not be up-to-date.
// see sigs.k8s.io/controller-runtime/pkg/client/split.go to understand how client
// works with this UncachedObjects filter.
func GetUncachedObjects() []client.Object {
	// client-side read cache reduces the number of requests processed in the API server,
	// which is good for performance. However, it can sometimes lead to obscure issues,
	// most notably lacking read-after-write consistency, i.e. reading a value immediately
	// after updating it may miss to see the changes.
	// while in most cases this problem can be mitigated by retrying later in an idempotent
	// manner, there are some cases where it cannot, for example if a decision is to be made
	// that has side-effect operations such as returning an error message to the user
	// (in webhook) or deleting certain resources (in controllerutil.HandleCRDeletion).
	// additionally, retry loops cause unnecessary delays when reconciliations are processed.
	// for the sake of performance, now only the objects created by the end-user is listed here,
	// to solve the two problems mentioned above.
	// consider carefully before adding new objects to this list.
	return []client.Object{
		// avoid to cache potential large data objects
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&appsv1alpha1.Cluster{},
		&appsv1alpha1.Configuration{},
	}
}

// Event is wrapper for Recorder.Event, if Recorder is nil, then it's no-op.
func (r *RequestCtx) Event(object runtime.Object, eventtype, reason, message string) {
	if r == nil || r.Recorder == nil {
		return
	}
	r.Recorder.Event(object, eventtype, reason, message)
}

// Eventf is wrapper for Recorder.Eventf, if Recorder is nil, then it's no-op.
func (r *RequestCtx) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	if r == nil || r.Recorder == nil {
		return
	}
	r.Recorder.Eventf(object, eventtype, reason, messageFmt, args...)
}

// UpdateCtxValue updates Context value, returns parent Context.
func (r *RequestCtx) UpdateCtxValue(key, val any) context.Context {
	p := r.Ctx
	r.Ctx = context.WithValue(r.Ctx, key, val)
	return p
}

// WithValue returns a copy of parent in which the value associated with key is
// val.
func (r *RequestCtx) WithValue(key, val any) context.Context {
	return context.WithValue(r.Ctx, key, val)
}

func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}

// MergeMetadataMapInplace merges two map[string]string, the targetMap will be updated.
func MergeMetadataMapInplace(originalMap map[string]string, targetMap *map[string]string) {
	if targetMap == nil || originalMap == nil {
		return
	}
	if *targetMap == nil {
		*targetMap = map[string]string{}
	}
	for k, v := range originalMap {
		// if the annotation not exist in targetAnnotations, copy it from original.
		if _, ok := (*targetMap)[k]; !ok {
			(*targetMap)[k] = v
		}
	}
}

// MergeMetadataMaps merges targetMaps into originalMap if item not exist in originalMap and return the merged map.
func MergeMetadataMaps(originalMap map[string]string, targetMaps ...map[string]string) map[string]string {
	mergeMap := map[string]string{}
	for k, v := range originalMap {
		mergeMap[k] = v
	}
	for _, targetMap := range targetMaps {
		for k, v := range targetMap {
			if _, ok := mergeMap[k]; !ok {
				mergeMap[k] = v
			}
		}
	}
	return mergeMap
}

var innerScheme, _ = appsv1alpha1.SchemeBuilder.Build()

func SetOwnerReference(owner, object metav1.Object) error {
	return controllerutil.SetOwnerReference(owner, object, innerScheme)
}

func SetControllerReference(owner, object metav1.Object) error {
	return controllerutil.SetControllerReference(owner, object, innerScheme)
}

func GeKubeRestConfig(userAgent string) *rest.Config {
	cfg := ctrl.GetConfigOrDie()
	clientQPS := viper.GetInt(constant.CfgClientQPS)
	if clientQPS != 0 {
		cfg.QPS = float32(clientQPS)
	}
	clientBurst := viper.GetInt(constant.CfgClientBurst)
	if clientBurst != 0 {
		cfg.Burst = clientBurst
	}
	if len(strings.TrimSpace(userAgent)) > 0 {
		rest.AddUserAgent(cfg, userAgent)
	}
	return cfg
}

// DeleteOwnedResources deletes the matched resources which are owned by the owner.
func DeleteOwnedResources[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](ctx context.Context,
	cli client.Client,
	owner client.Object,
	resourceMatchLabels client.MatchingLabels,
	_ func(T, PT, L, PL)) error {
	var objList L
	if err := cli.List(ctx, PL(&objList), client.InNamespace(owner.GetNamespace()), resourceMatchLabels); err != nil {
		return err
	}
	items := reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T)
	for _, obj := range items {
		pobj := PT(&obj)
		for _, v := range pobj.GetOwnerReferences() {
			if v.UID != owner.GetUID() {
				continue
			}
			if err := BackgroundDeleteObject(cli, ctx, pobj); err != nil {
				return err
			}
		}
	}
	return nil
}

func MergeList[E any](src, dst *[]E, f func(E) func(E) bool) {
	if len(*src) == 0 {
		return
	}
	for i := range *src {
		item := (*src)[i]
		index := slices.IndexFunc(*dst, f(item))
		if index >= 0 {
			(*dst)[index] = item
		} else {
			*dst = append(*dst, item)
		}
	}
}
