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

package preflight

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	pkgcollector "github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"github.com/spf13/viper"
	"helm.sh/helm/v3/pkg/cli/values"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	kbcollector "github.com/apecloud/kubeblocks/internal/preflight/collector"
)

func CollectPreflight(f cmdutil.Factory, helmOpts *values.Options, ctx context.Context, kbPreflight *preflightv1beta2.Preflight, kbHostPreflight *preflightv1beta2.HostPreflight, progressCh chan interface{}) ([]preflight.CollectResult, error) {
	var (
		collectResults []preflight.CollectResult
		err            error
	)
	// deal with preflight
	if kbPreflight != nil && (len(kbPreflight.Spec.ExtendCollectors) > 0 || len(kbPreflight.Spec.Collectors) > 0) {
		res, err := CollectClusterData(ctx, kbPreflight, f, helmOpts, progressCh)
		if err != nil {
			return collectResults, errors.Wrap(err, "failed to collect data in cluster")
		}
		collectResults = append(collectResults, *res)
	}
	// deal with hostPreflight
	if kbHostPreflight != nil {
		if len(kbHostPreflight.Spec.ExtendCollectors) > 0 || len(kbHostPreflight.Spec.Collectors) > 0 {
			res, err := CollectHostData(ctx, kbHostPreflight, progressCh)
			if err != nil {
				return collectResults, errors.Wrap(err, "failed to collect data from extend host")
			}
			collectResults = append(collectResults, *res)
		}
		if len(kbHostPreflight.Spec.RemoteCollectors) > 0 {
			res, err := CollectRemoteData(ctx, kbHostPreflight, f, progressCh)
			if err != nil {
				return collectResults, errors.Wrap(err, "failed to collect data remotely")
			}
			collectResults = append(collectResults, *res)
		}
	}
	return collectResults, err
}

// CollectHostData transforms the specs of hostPreflight to HostCollector, and sets the collectOpts
func CollectHostData(ctx context.Context, hostPreflight *preflightv1beta2.HostPreflight, progressCh chan interface{}) (*preflight.CollectResult, error) {
	collectOpts := preflight.CollectOpts{
		ProgressChan: progressCh,
	}
	var collectors []pkgcollector.HostCollector
	for _, collectSpec := range hostPreflight.Spec.Collectors {
		collector, ok := pkgcollector.GetHostCollector(collectSpec, "")
		if ok {
			collectors = append(collectors, collector)
		}
	}
	for _, kbCollector := range hostPreflight.Spec.ExtendCollectors {
		collector, ok := kbcollector.GetExtendHostCollector(kbCollector, "")
		if ok {
			collectors = append(collectors, collector)
		}
	}
	collectResults, err := CollectHost(ctx, collectOpts, collectors, hostPreflight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect from extend host")
	}
	return &collectResults, nil
}

// CollectHost collects host data against by HostCollector, and returns the collected data which is encapsulated in CollectResult struct
func CollectHost(ctx context.Context, opts preflight.CollectOpts, collectors []pkgcollector.HostCollector, hostPreflight *preflightv1beta2.HostPreflight) (preflight.CollectResult, error) {
	allCollectedData := make(map[string][]byte)
	collectResult := KBHostCollectResult{
		HostCollectResult: preflight.HostCollectResult{
			Collectors: collectors,
			Context:    ctx,
		},
		AnalyzerSpecs:   hostPreflight.Spec.Analyzers,
		KbAnalyzerSpecs: hostPreflight.Spec.ExtendAnalyzers,
	}
	for _, collector := range collectors {
		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			continue
		}
		opts.ProgressChan <- fmt.Sprintf("[%s] Running collector...", collector.Title())
		result, err := collector.Collect(opts.ProgressChan)
		if err != nil {
			opts.ProgressChan <- errors.Errorf("failed to run collector: %s: %v", collector.Title(), err)
		}
		for k, v := range result {
			allCollectedData[k] = v
		}
	}
	collectResult.AllCollectedData = allCollectedData
	return collectResult, nil
}

// CollectClusterData transforms the specs of Preflight to Collector, and sets the collectOpts, such as restConfig, Namespace, and ProgressChan
func CollectClusterData(ctx context.Context, kbPreflight *preflightv1beta2.Preflight, f cmdutil.Factory, helmOpts *values.Options, progressCh chan interface{}) (*preflight.CollectResult, error) {
	var err error
	v := viper.GetViper()

	collectOpts := preflight.CollectOpts{
		Namespace:              v.GetString("namespace"),
		IgnorePermissionErrors: v.GetBool("collect-without-permissions"),
		ProgressChan:           progressCh,
	}

	if collectOpts.KubernetesRestConfig, err = f.ToRESTConfig(); err != nil {
		return nil, errors.Wrap(err, "failed to instantiate Kubernetes restconfig")
	}

	k8sClient, err := f.KubernetesClientSet()
	if err != nil {
		return nil, errors.Wrap(err, "failed to instantiate Kubernetes client")
	}

	if v.GetString("since") != "" || v.GetString("since-time") != "" {
		err := ParseTimeFlags(v.GetString("since-time"), v.GetString("since"), kbPreflight.Spec.Collectors)
		if err != nil {
			return nil, err
		}
	}

	collectSpecs := make([]*troubleshoot.Collect, 0, len(kbPreflight.Spec.Collectors))
	collectSpecs = append(collectSpecs, kbPreflight.Spec.Collectors...)
	collectSpecs = pkgcollector.EnsureCollectorInList(
		collectSpecs, troubleshoot.Collect{ClusterInfo: &troubleshoot.ClusterInfo{}},
	)
	collectSpecs = pkgcollector.EnsureCollectorInList(
		collectSpecs, troubleshoot.Collect{ClusterResources: &troubleshoot.ClusterResources{}},
	)
	collectSpecs = pkgcollector.DedupCollectors(collectSpecs)
	collectSpecs = pkgcollector.EnsureClusterResourcesFirst(collectSpecs)

	var collectors []pkgcollector.Collector
	allCollectorsMap := make(map[reflect.Type][]pkgcollector.Collector)
	for _, collectSpec := range collectSpecs {
		if collectorInterface, ok := pkgcollector.GetCollector(collectSpec, "", collectOpts.Namespace, collectOpts.KubernetesRestConfig, k8sClient, nil); ok {
			if collector, ok := collectorInterface.(pkgcollector.Collector); ok {
				err := collector.CheckRBAC(ctx, collector, collectSpec, collectOpts.KubernetesRestConfig, collectOpts.Namespace)
				if err != nil {
					return nil, errors.Wrap(err, "failed to check RBAC for collectors")
				}
				collectorType := reflect.TypeOf(collector)
				allCollectorsMap[collectorType] = append(allCollectorsMap[collectorType], collector)
			}
		}
	}
	// for _, collectSpec := range kbPreflight.Spec.ExtendCollectors {
	//	// todo user defined cluster collector
	// }

	collectResults, err := CollectCluster(ctx, collectOpts, collectors, allCollectorsMap, kbPreflight, helmOpts)
	return &collectResults, err
}

// CollectCluster collects cluster data against by Collector, and returns the collected data which is encapsulated in CollectResult struct
func CollectCluster(ctx context.Context,
	opts preflight.CollectOpts,
	allCollectors []pkgcollector.Collector,
	allCollectorsMap map[reflect.Type][]pkgcollector.Collector,
	kbPreflight *preflightv1beta2.Preflight,
	helmOpts *values.Options,
) (preflight.CollectResult, error) {
	var foundForbidden bool
	allCollectedData := make(map[string][]byte)
	collectorList := map[string]preflight.CollectorStatus{}
	for _, collectors := range allCollectorsMap {
		if mergeCollector, ok := collectors[0].(pkgcollector.MergeableCollector); ok {
			mergedCollectors, err := mergeCollector.Merge(collectors)
			if err != nil {
				msg := fmt.Sprintf("failed to merge collector: %s: %s", mergeCollector.Title(), err)
				opts.ProgressChan <- msg
			}
			allCollectors = append(allCollectors, mergedCollectors...)
		} else {
			allCollectors = append(allCollectors, collectors...)
		}

		for _, collector := range collectors {
			for _, e := range collector.GetRBACErrors() {
				foundForbidden = true
				opts.ProgressChan <- e
			}

			// generate a map of all collectors for atomic status messages
			collectorList[collector.Title()] = preflight.CollectorStatus{
				Status: "pending",
			}
		}
	}

	collectResult := KBClusterCollectResult{
		ClusterCollectResult: preflight.ClusterCollectResult{
			Collectors: allCollectors,
			Context:    ctx,
		},
		AnalyzerSpecs:   kbPreflight.Spec.Analyzers,
		KbAnalyzerSpecs: kbPreflight.Spec.ExtendAnalyzers,
		HelmOptions:     helmOpts,
	}

	if foundForbidden && !opts.IgnorePermissionErrors {
		// collectResult.IsRBACAllowed() = false
		return collectResult, errors.New("insufficient permissions to run all collectors")
	}

	for i, collector := range allCollectors {
		isExcluded, _ := collector.IsExcluded()
		if isExcluded {
			logger.Printf("Excluding %q collector", collector.Title())
			continue
		}

		// skip collectors with RBAC errors unless its the ClusterResources collector
		if collector.HasRBACErrors() {
			if _, ok := collector.(*pkgcollector.CollectClusterResources); !ok {
				opts.ProgressChan <- fmt.Sprintf("skipping collector %s with insufficient RBAC permissions", collector.Title())
				opts.ProgressChan <- preflight.CollectProgress{
					CurrentName:    collector.Title(),
					CurrentStatus:  "skipped",
					CompletedCount: i + 1,
					TotalCount:     len(allCollectors),
					Collectors:     collectorList,
				}
				continue
			}
		}

		collectorList[collector.Title()] = preflight.CollectorStatus{
			Status: "running",
		}
		opts.ProgressChan <- preflight.CollectProgress{
			CurrentName:    collector.Title(),
			CurrentStatus:  "running",
			CompletedCount: i,
			TotalCount:     len(allCollectors),
			Collectors:     collectorList,
		}

		result, err := collector.Collect(opts.ProgressChan)
		if err != nil {
			collectorList[collector.Title()] = preflight.CollectorStatus{
				Status: "failed",
			}
			opts.ProgressChan <- errors.Errorf("failed to run collector: %s: %v", collector.Title(), err)
			opts.ProgressChan <- preflight.CollectProgress{
				CurrentName:    collector.Title(),
				CurrentStatus:  "failed",
				CompletedCount: i + 1,
				TotalCount:     len(allCollectors),
				Collectors:     collectorList,
			}
			continue
		}

		collectorList[collector.Title()] = preflight.CollectorStatus{
			Status: "completed",
		}
		opts.ProgressChan <- preflight.CollectProgress{
			CurrentName:    collector.Title(),
			CurrentStatus:  "completed",
			CompletedCount: i + 1,
			TotalCount:     len(allCollectors),
			Collectors:     collectorList,
		}

		for k, v := range result {
			allCollectedData[k] = v
		}
	}

	collectResult.AllCollectedData = allCollectedData
	return collectResult, nil
}

func CollectRemoteData(ctx context.Context, preflightSpec *preflightv1beta2.HostPreflight, f cmdutil.Factory, progressCh chan interface{}) (*preflight.CollectResult, error) {
	v := viper.GetViper()

	restConfig, err := f.ToRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	labelSelector, err := labels.Parse(v.GetString("selector"))
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse selector")
	}

	namespace := v.GetString("namespace")
	if namespace == "" {
		namespace = "default"
	}

	timeout := v.GetDuration("request-timeout")
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	collectOpts := preflight.CollectOpts{
		Namespace:              namespace,
		IgnorePermissionErrors: v.GetBool("collect-without-permissions"),
		ProgressChan:           progressCh,
		KubernetesRestConfig:   restConfig,
		Image:                  v.GetString("collector-image"),
		PullPolicy:             v.GetString("collector-pullpolicy"),
		LabelSelector:          labelSelector.String(),
		Timeout:                timeout,
	}

	collectResults, err := preflight.CollectRemoteWithContext(ctx, collectOpts, ExtractHostPreflightSpec(preflightSpec))
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect from remote")
	}

	return &collectResults, nil
}

func ParseTimeFlags(sinceTimeStr, sinceStr string, collectors []*troubleshoot.Collect) error {
	var (
		sinceTime time.Time
		err       error
	)
	if sinceTimeStr != "" {
		if sinceStr != "" {
			return errors.Errorf("at most one of `sinceTime` or `since` may be specified")
		}
		sinceTime, err = time.Parse(time.RFC3339, sinceTimeStr)
		if err != nil {
			return errors.Wrap(err, "unable to parse --since-time flag")
		}
	} else {
		parsedDuration, err := time.ParseDuration(sinceStr)
		if err != nil {
			return errors.Wrap(err, "unable to parse --since flag")
		}
		now := time.Now()
		sinceTime = now.Add(0 - parsedDuration)
	}
	for _, collector := range collectors {
		if collector.Logs != nil {
			if collector.Logs.Limits == nil {
				collector.Logs.Limits = new(troubleshoot.LogLimits)
			}
			collector.Logs.Limits.SinceTime = metav1.NewTime(sinceTime)
		}
	}
	return nil
}
