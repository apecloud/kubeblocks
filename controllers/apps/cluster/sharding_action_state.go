/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

KubeBlocks is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

KubeBlocks is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with KubeBlocks.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

const (
	shardingActionStateRefVersion = 1
	// Keep every ConfigMap comfortably below the API server object size limit.
	shardingActionStateChunkSize = 512 * 1024
	shardingActionStateDataKey   = "state"
	shardingActionStateMaxChunks = 16 * 1024
)

type shardingActionStateRef struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	Chunks  int    `json:"chunks"`
}

func shardingActionStateName(source *appsv1.Component, annotation string) string {
	identity := string(source.UID)
	if identity == "" {
		identity = source.Namespace + "/" + source.Name
	}
	sum := sha256.Sum256([]byte(identity + "\x00" + annotation))
	return "kb-shard-action-" + hex.EncodeToString(sum[:8])
}

func shardingActionStateChunkName(base string, index int) string {
	return fmt.Sprintf("%s-%04d", base, index)
}

func getShardingActionStateRef(source *appsv1.Component, annotation string) (*shardingActionStateRef, bool, error) {
	value, found := source.Annotations[annotation]
	if !found {
		return nil, false, nil
	}
	ref := &shardingActionStateRef{}
	if err := json.Unmarshal([]byte(value), ref); err != nil {
		return nil, false, fmt.Errorf("invalid %s annotation on component %s: %w", annotation, source.Name, err)
	}
	if ref.Version != shardingActionStateRefVersion {
		return nil, false, fmt.Errorf("invalid %s annotation on component %s: unsupported version %d",
			annotation, source.Name, ref.Version)
	}
	if len(validation.IsDNS1123Subdomain(ref.Name)) != 0 ||
		ref.Chunks <= 0 || ref.Chunks > shardingActionStateMaxChunks {
		return nil, false, fmt.Errorf("invalid %s annotation on component %s: invalid state reference",
			annotation, source.Name)
	}
	return ref, true, nil
}

func findShardingActionStateChunk(transCtx *clusterTransformContext, dag *graph.DAG,
	key types.NamespacedName) (*corev1.ConfigMap, bool, error) {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	if dag != nil {
		if vertex := graphCli.FindMatchedVertex(dag, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		}); vertex != nil {
			return vertex.(*model.ObjectVertex).Obj.(*corev1.ConfigMap), true, nil
		}
	}
	cm := &corev1.ConfigMap{}
	if err := transCtx.Client.Get(transCtx.Context, key, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return cm, false, nil
}

func getShardingActionState(transCtx *clusterTransformContext, dag *graph.DAG,
	source *appsv1.Component, annotation string) (*shardingActionTargets, bool, error) {
	ref, found, err := getShardingActionStateRef(source, annotation)
	if err != nil || !found {
		return nil, found, err
	}

	var data bytes.Buffer
	for i := 0; i < ref.Chunks; i++ {
		name := shardingActionStateChunkName(ref.Name, i)
		cm, _, err := findShardingActionStateChunk(transCtx, dag,
			types.NamespacedName{Namespace: source.Namespace, Name: name})
		if err != nil {
			return nil, false, err
		}
		if cm == nil {
			return nil, false, fmt.Errorf("sharding action state chunk %s is not found", name)
		}
		chunk, ok := cm.BinaryData[shardingActionStateDataKey]
		if !ok {
			return nil, false, fmt.Errorf("sharding action state chunk %s has no %q data", name,
				shardingActionStateDataKey)
		}
		_, _ = data.Write(chunk)
	}

	state := &shardingActionTargets{}
	if err := json.Unmarshal(data.Bytes(), state); err != nil {
		return nil, false, fmt.Errorf("invalid sharding action state for component %s: %w", source.Name, err)
	}
	if err := validateShardingActionTargets(state); err != nil {
		return nil, false, fmt.Errorf("invalid sharding action state for component %s: %w", source.Name, err)
	}
	return state, true, nil
}

func setShardingActionState(transCtx *clusterTransformContext, dag *graph.DAG,
	source *appsv1.Component, annotation string, state *shardingActionTargets) error {
	sortShardingActionTargets(state)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	oldRef, found, err := getShardingActionStateRef(source, annotation)
	if err != nil {
		return err
	}
	ref := &shardingActionStateRef{
		Version: shardingActionStateRefVersion,
		Name:    shardingActionStateName(source, annotation),
		Chunks:  (len(data) + shardingActionStateChunkSize - 1) / shardingActionStateChunkSize,
	}
	if found {
		ref.Name = oldRef.Name
	}
	if ref.Chunks == 0 {
		ref.Chunks = 1
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	for i := 0; i < ref.Chunks; i++ {
		begin := i * shardingActionStateChunkSize
		end := min(begin+shardingActionStateChunkSize, len(data))
		chunk := append([]byte(nil), data[begin:end]...)
		key := types.NamespacedName{
			Namespace: source.Namespace,
			Name:      shardingActionStateChunkName(ref.Name, i),
		}
		cm, inGraph, err := findShardingActionStateChunk(transCtx, dag, key)
		if err != nil {
			return err
		}
		if cm == nil {
			controller := true
			blockOwnerDeletion := true
			cm = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name,
					Labels: map[string]string{
						constant.AppManagedByLabelKey: constant.AppName,
						constant.AppInstanceLabelKey:  transCtx.Cluster.Name,
					},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         appsv1.GroupVersion.String(),
						Kind:               appsv1.ComponentKind,
						Name:               source.Name,
						UID:                source.UID,
						Controller:         &controller,
						BlockOwnerDeletion: &blockOwnerDeletion,
					}},
				},
				BinaryData: map[string][]byte{shardingActionStateDataKey: chunk},
			}
			graphCli.Create(dag, cm)
			continue
		}
		if reflect.DeepEqual(cm.BinaryData[shardingActionStateDataKey], chunk) {
			continue
		}
		if inGraph {
			if cm.BinaryData == nil {
				cm.BinaryData = map[string][]byte{}
			}
			cm.BinaryData[shardingActionStateDataKey] = chunk
		} else {
			updated := cm.DeepCopy()
			if updated.BinaryData == nil {
				updated.BinaryData = map[string][]byte{}
			}
			updated.BinaryData[shardingActionStateDataKey] = chunk
			graphCli.Update(dag, cm, updated)
		}
	}

	if found {
		for i := ref.Chunks; i < oldRef.Chunks; i++ {
			key := types.NamespacedName{
				Namespace: source.Namespace,
				Name:      shardingActionStateChunkName(oldRef.Name, i),
			}
			cm, _, err := findShardingActionStateChunk(transCtx, dag, key)
			if err != nil {
				return err
			}
			if cm != nil {
				graphCli.Delete(dag, cm)
			}
		}
	}

	value, err := json.Marshal(ref)
	if err != nil {
		return err
	}
	if source.Annotations == nil {
		source.Annotations = map[string]string{}
	}
	source.Annotations[annotation] = string(value)
	return nil
}

func deleteShardingActionState(transCtx *clusterTransformContext, dag *graph.DAG,
	source *appsv1.Component, annotation string) error {
	ref, found, err := getShardingActionStateRef(source, annotation)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for i := 0; i < ref.Chunks; i++ {
		key := types.NamespacedName{
			Namespace: source.Namespace,
			Name:      shardingActionStateChunkName(ref.Name, i),
		}
		cm, _, err := findShardingActionStateChunk(transCtx, dag, key)
		if err != nil {
			return err
		}
		if cm != nil {
			graphCli.Delete(dag, cm)
		}
	}
	delete(source.Annotations, annotation)
	return nil
}

func sortShardingActionTargets(targets *shardingActionTargets) {
	// Keep state deterministic so equivalent updates do not rewrite ConfigMaps.
	for i := range targets.Targets {
		sortShardingActionTargetPods(targets.Targets[i].Pods)
	}
	sortShardingActionTargetComponents(targets.Targets)
}

func sortShardingActionTargetComponents(targets []shardingActionTarget) {
	// Kept as a small helper to avoid exporting the persistence representation.
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Component < targets[j].Component
	})
}

func sortShardingActionTargetPods(pods []shardingActionTargetPod) {
	sort.Slice(pods, func(i, j int) bool {
		return pods[i].Name < pods[j].Name
	})
}
