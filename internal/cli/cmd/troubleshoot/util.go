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

package troubleshoot

import (
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ConcatPreflightSpec splices multiple PreflightSpec into one Preflight
func ConcatPreflightSpec(target *troubleshootv1beta2.Preflight, source *troubleshootv1beta2.Preflight) *troubleshootv1beta2.Preflight {
	if target == nil && source != nil {
		return source
	}
	newSpec := target.DeepCopy()
	newSpec.Spec.Collectors = append(newSpec.Spec.Collectors, source.Spec.Collectors...)
	newSpec.Spec.RemoteCollectors = append(newSpec.Spec.RemoteCollectors, source.Spec.RemoteCollectors...)
	newSpec.Spec.Analyzers = append(newSpec.Spec.Analyzers, source.Spec.Analyzers...)
	return newSpec
}

// ConcatHostPreflightSpec splices multiple HostPreflightSpec into one HostPreflight
func ConcatHostPreflightSpec(target *troubleshootv1beta2.HostPreflight, source *troubleshootv1beta2.HostPreflight) *troubleshootv1beta2.HostPreflight {
	if target == nil && source != nil {
		return source
	}
	newSpec := target.DeepCopy()
	newSpec.Spec.Collectors = append(newSpec.Spec.Collectors, source.Spec.Collectors...)
	newSpec.Spec.RemoteCollectors = append(newSpec.Spec.RemoteCollectors, source.Spec.RemoteCollectors...)
	newSpec.Spec.Analyzers = append(newSpec.Spec.Analyzers, source.Spec.Analyzers...)
	return newSpec
}

func collectDataInCluster(preflightSpec *troubleshootv1beta2.Preflight, progressCh chan interface{}, p preflightOptions) (*preflight.CollectResult, error) {
	restConfig, err := p.factory.ToRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}
	collectOpts := preflight.CollectOpts{
		Namespace:              p.namespace,
		IgnorePermissionErrors: *p.CollectWithoutPermissions,
		ProgressChan:           progressCh,
		KubernetesRestConfig:   restConfig,
	}
	if *p.Since != "" || *p.SinceTime != "" {
		err := parseTimeFlags(*p.Since, *p.SinceTime, preflightSpec.Spec.Collectors)
		if err != nil {
			return nil, err
		}
	}
	collectResults, err := preflight.Collect(collectOpts, preflightSpec)
	if err != nil {
		return nil, err
	}
	return &collectResults, nil
}

func collectRemoteData(preflightSpec *troubleshootv1beta2.HostPreflight, progressCh chan interface{}, p preflightOptions) (*preflight.CollectResult, error) {
	restConfig, err := p.factory.ToRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}
	labelSelector, err := labels.Parse(*p.Selector)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse selector")
	}
	namespace, _, err := p.factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, err
	}
	timeout := 30 * time.Second
	collectOpts := preflight.CollectOpts{
		Namespace:              namespace,
		IgnorePermissionErrors: *p.CollectWithoutPermissions,
		ProgressChan:           progressCh,
		KubernetesRestConfig:   restConfig,
		Image:                  *p.CollectorImage,
		PullPolicy:             *p.CollectorPullPolicy,
		LabelSelector:          labelSelector.String(),
		Timeout:                timeout,
	}
	collectResults, err := preflight.CollectRemote(collectOpts, preflightSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect data from remote")
	}
	return &collectResults, nil
}

func collectHostData(hostPreflightSpec *troubleshootv1beta2.HostPreflight, progressCh chan interface{}) (*preflight.CollectResult, error) {
	collectOpts := preflight.CollectOpts{
		ProgressChan: progressCh,
	}
	collectResults, err := preflight.CollectHost(collectOpts, hostPreflightSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect data from host")
	}
	return &collectResults, nil
}

func parseTimeFlags(sinceStr, sinceTimeStr string, collectors []*troubleshootv1beta2.Collect) error {
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
				collector.Logs.Limits = new(troubleshootv1beta2.LogLimits)
			}
			collector.Logs.Limits.SinceTime = metav1.NewTime(sinceTime)
		}
	}
	return nil
}
