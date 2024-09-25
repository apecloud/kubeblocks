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
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

const (
	GenerateNameMaxRetryTimes = 1000000
	characters                = "abcdefghijklmnopqrstuvwxyz0123456789"
	suffixLength              = 3
	maxCombinations           = 36 * 36 * 36
)

var mu sync.Mutex

// GenShardingCompSpecList generates sharding component specs list based on the sharding spec.
// TODO: generate sharding component name with stable identity
func GenShardingCompSpecList(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, shardingSpec *appsv1.ShardingSpec) ([]*appsv1.ClusterComponentSpec, error) {
	mu.Lock()
	defer mu.Unlock()

	existingShardingCompNames, err := getShardingCompNamesFromAnnotations(cluster, shardingSpec.Name)
	if err != nil {
		return nil, err
	}

	// list undeleted sharding component specs, the deleting ones are not included
	undeletedShardingCompSpecs, err := listUndeletedShardingCompSpecs(ctx, cli, cluster, shardingSpec)
	if err != nil {
		return nil, err
	}

	// make sure the sharding component names in annotations are consistent with undeleted sharding components
	if len(existingShardingCompNames) != 0 && len(undeletedShardingCompSpecs) != len(existingShardingCompNames) {
		updatedShardingCompNames := make([]string, 0)
		for _, v := range undeletedShardingCompSpecs {
			updatedShardingCompNames = append(updatedShardingCompNames, v.Name)
		}
		if err := updateShardingCompNamesToAnnotations(cluster, shardingSpec.Name, updatedShardingCompNames); err != nil {
			return nil, err
		}
		return nil, NewErrorf(ErrorTypeRequeue, "requeue to waiting for sharding component name annotation to be updated")
	}

	for _, v := range undeletedShardingCompSpecs {
		// if spec.name not in existingShardingCompNames, add it
		if !sets.NewString(existingShardingCompNames...).Has(v.Name) {
			existingShardingCompNames = append(existingShardingCompNames, v.Name)
		}
	}

	compSpecList := make([]*appsv1.ClusterComponentSpec, 0)
	compSpecList = append(compSpecList, undeletedShardingCompSpecs...)
	shardTpl := shardingSpec.Template
	switch {
	case len(undeletedShardingCompSpecs) == int(shardingSpec.Shards):
		return undeletedShardingCompSpecs, nil
	case len(undeletedShardingCompSpecs) < int(shardingSpec.Shards):
		neededShards := int(shardingSpec.Shards) - len(undeletedShardingCompSpecs)
		newNames, err := GenerateUniqueRandomStrings(neededShards, existingShardingCompNames)
		if err != nil {
			return nil, err
		}
		newShardingCompNames := make([]string, 0, neededShards)
		for _, newName := range newNames {
			shardClusterCompSpec := shardTpl.DeepCopy()
			shardClusterCompSpec.Name = fmt.Sprintf("%s-%s", shardingSpec.Name, newName)
			newShardingCompNames = append(newShardingCompNames, shardClusterCompSpec.Name)
			compSpecList = append(compSpecList, shardClusterCompSpec)
		}

		// Update existing sharding component names to annotations
		existingShardingCompNames = append(existingShardingCompNames, newShardingCompNames...)
		if err := updateShardingCompNamesToAnnotations(cluster, shardingSpec.Name, existingShardingCompNames); err != nil {
			return nil, err
		}
	case len(undeletedShardingCompSpecs) > int(shardingSpec.Shards):
		// TODO: order by?
		compSpecList = compSpecList[:int(shardingSpec.Shards)]
	}
	return compSpecList, nil
}

func getShardingCompNamesFromAnnotations(cluster *appsv1.Cluster, shardingName string) ([]string, error) {
	if annotations, ok := cluster.Annotations[constant.GetShardingCompsAnnotationKey(shardingName)]; ok {
		var compNames []string
		if err := json.Unmarshal([]byte(annotations), &compNames); err != nil {
			return nil, fmt.Errorf("error unmarshalling existing sharding components from annotations: %v", err)
		}
		return compNames, nil
	}
	return nil, nil
}

func updateShardingCompNamesToAnnotations(cluster *appsv1.Cluster, shardingName string, compNames []string) error {
	compNamesAnnotations, err := json.Marshal(compNames)
	if err != nil {
		return fmt.Errorf("error marshalling sharding components to annotations: %v", err)
	}
	cluster.Annotations[constant.GetShardingCompsAnnotationKey(shardingName)] = string(compNamesAnnotations)
	return nil
}

// listShardingComponents lists sharding components and checks if the sharding components are correct. It returns undeleted and deleting sharding components.
func listShardingComponents(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, shardingSpec *appsv1.ShardingSpec) ([]appsv1.Component, []appsv1.Component, error) {
	shardingComps, err := ListShardingComponents(ctx, cli, cluster, shardingSpec.Name)
	if err != nil {
		return nil, nil, err
	}

	deletingShardingComps := make([]appsv1.Component, 0)
	undeletedShardingComps := make([]appsv1.Component, 0)
	for _, comp := range shardingComps {
		if comp.GetDeletionTimestamp().IsZero() {
			undeletedShardingComps = append(undeletedShardingComps, comp)
		} else {
			deletingShardingComps = append(deletingShardingComps, comp)
		}
	}

	return undeletedShardingComps, deletingShardingComps, nil
}

func ListShardingComponents(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, shardingName string) ([]appsv1.Component, error) {
	compList := &appsv1.ComponentList{}
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:       cluster.Name,
		constant.KBAppShardingNameLabelKey: shardingName,
	}
	if err := cli.List(ctx, compList, client.InNamespace(cluster.Namespace), ml); err != nil {
		return nil, err
	}
	return compList.Items, nil
}

// listUndeletedShardingCompSpecs lists undeleted sharding component specs.
func listUndeletedShardingCompSpecs(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster, shardingSpec *appsv1.ShardingSpec) ([]*appsv1.ClusterComponentSpec, error) {
	return listShardingCompSpecs(ctx, cli, cluster, shardingSpec, false)
}

// listAllShardingCompSpecs lists all sharding component specs, including undeleted and deleting ones.
func listAllShardingCompSpecs(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster, shardingSpec *appsv1.ShardingSpec) ([]*appsv1.ClusterComponentSpec, error) {
	return listShardingCompSpecs(ctx, cli, cluster, shardingSpec, true)
}

// listShardingCompSpecs lists sharding component specs, with an option to include those marked for deletion.
func listShardingCompSpecs(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster, shardingSpec *appsv1.ShardingSpec, includeDeleting bool) ([]*appsv1.ClusterComponentSpec, error) {
	if shardingSpec == nil {
		return nil, nil
	}

	undeletedShardingComps, deletingShardingComps, err := listShardingComponents(ctx, cli, cluster, shardingSpec)
	if err != nil {
		return nil, err
	}

	compSpecList := make([]*appsv1.ClusterComponentSpec, 0, len(undeletedShardingComps)+len(deletingShardingComps))
	shardTpl := shardingSpec.Template

	processComps := func(comps []appsv1.Component) error {
		for _, comp := range comps {
			compShortName, err := parseCompShortName(cluster.Name, comp.Name)
			if err != nil {
				return err
			}
			shardClusterCompSpec := shardTpl.DeepCopy()
			shardClusterCompSpec.Name = compShortName
			compSpecList = append(compSpecList, shardClusterCompSpec)
		}
		return nil
	}

	err = processComps(undeletedShardingComps)
	if err != nil {
		return nil, err
	}

	if includeDeleting {
		err = processComps(deletingShardingComps)
		if err != nil {
			return nil, err
		}
	}

	return compSpecList, nil
}

func parseCompShortName(clusterName, compName string) (string, error) {
	name, found := strings.CutPrefix(compName, fmt.Sprintf("%s-", clusterName))
	if !found {
		return "", fmt.Errorf("the component name has no cluster name as prefix: %s", compName)
	}
	return name, nil
}

// GenerateUniqueRandomStrings generates a set of unique random strings based on existing strings.
func GenerateUniqueRandomStrings(count int, existing []string) ([]string, error) {
	if count > maxCombinations {
		return nil, fmt.Errorf("cannot generate more than %d unique strings", maxCombinations)
	}

	bitmap := make([]bool, maxCombinations)
	resultSet := make(map[string]struct{})

	for _, str := range existing {
		index := stringToIndex(str)
		if index != -1 {
			bitmap[index] = true
			resultSet[str] = struct{}{}
		}
	}

	result := make([]string, len(existing))
	copy(result, existing)

	for len(result) < count {
		index := generateUniqueIndex(bitmap)
		if index != -1 {
			bitmap[index] = true
			newStr := indexToString(index)
			result = append(result, newStr)
			resultSet[newStr] = struct{}{}
		}
	}

	return result, nil
}

func generateUniqueIndex(bitmap []bool) int {
	for {
		index := rand.Intn(maxCombinations)
		if !bitmap[index] {
			return index
		}
	}
}

func stringToIndex(s string) int {
	if len(s) != suffixLength {
		return -1
	}
	index := 0
	for i := 0; i < suffixLength; i++ {
		index = index*len(characters) + indexOf(characters, s[i])
	}
	return index
}

func indexToString(index int) string {
	result := make([]byte, suffixLength)
	for i := suffixLength - 1; i >= 0; i-- {
		result[i] = characters[index%len(characters)]
		index /= len(characters)
	}
	return string(result)
}

func indexOf(s string, char byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == char {
			return i
		}
	}
	return -1
}
