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

package component

import (
	"errors"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// clusterConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.Cluster.
type clusterConvertor struct{}

// compDefConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.CompDef.
type compDefConvertor struct{}

// classDefRefConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.ClassDefRef.
type classDefRefConvertor struct{}

// serviceRefConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.ServiceRefs.
type serviceRefConvertor struct{}

// resourceConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.Resources.
type resourceConvertor struct{}

// volumeClaimTemplateConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.VolumeClaimTemplates.
type volumeClaimTemplateConvertor struct{}

// replicasConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.Replicas.
type replicasConvertor struct{}

// configConvertor2 is an implementation of the convertor interface, used to convert the given object into Component.Spec.Configs.
type configConvertor2 struct{}

// monitorConvertor2 is an implementation of the convertor interface, used to convert the given object into Component.Spec.Monitor.
type monitorConvertor2 struct{}

// enabledLogConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.EnabledLogs.
type enabledLogConvertor struct{}

// updateStrategyConvertor2 is an implementation of the convertor interface, used to convert the given object into Component.Spec.UpdateStrategy.
type updateStrategyConvertor2 struct{}

// serviceAccountNameConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.ServiceAccountName.
type serviceAccountNameConvertor struct{}

// affinityConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.Affinity.
type affinityConvertor struct{}

// tolerationConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.Tolerations.
type tolerationConvertor struct{}

// tlsConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.TLS.
type tlsConvertor struct{}

// issuerConvertor is an implementation of the convertor interface, used to convert the given object into Component.Spec.Issuer.
type issuerConvertor struct{}

// buildComponentFrom builds a new Component object based on Cluster, ClusterComponentDefinition, ClusterComponentVersion and ClusterComponentSpec.
func buildComponentFrom(cluster *appsv1alpha1.Cluster,
	clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterCompVer *appsv1alpha1.ClusterComponentVersion,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.Component, error) {
	if clusterCompDef == nil || clusterCompSpec == nil {
		return nil, nil
	}
	convertors := map[string]convertor{
		"cluster":              &clusterConvertor{},
		"compdef":              &compDefConvertor{},
		"classdefref":          &classDefRefConvertor{},
		"servicerefs":          &serviceRefConvertor{},
		"resources":            &resourceConvertor{},
		"volumeclaimtemplates": &volumeClaimTemplateConvertor{},
		"replicas":             &replicasConvertor{},
		"configs":              &configConvertor2{},
		"monitor":              &monitorConvertor2{},
		"enabledlogs":          &enabledLogConvertor{},
		"updatestrategy":       &updateStrategyConvertor2{},
		"serviceaccountname":   &serviceAccountNameConvertor{},
		"affinity":             &affinityConvertor{},
		"tolerations":          &tolerationConvertor{},
		"tls":                  &tlsConvertor{},
		"issuer":               &issuerConvertor{},
	}
	comp := &appsv1alpha1.Component{}
	if err := covertObject(convertors, &comp.Spec, cluster, clusterCompDef, clusterCompVer, clusterCompSpec); err != nil {
		return nil, err
	}
	return comp, nil
}

// parseComponentConvertorArgs parses the args of component convertor.
func parseComponentConvertorArgs(args ...any) (*appsv1alpha1.Cluster, *appsv1alpha1.ClusterComponentDefinition, *appsv1alpha1.ClusterComponentVersion, *appsv1alpha1.ClusterComponentSpec, error) {
	cluster, ok := args[0].(*appsv1alpha1.Cluster)
	if !ok {
		return nil, nil, nil, nil, errors.New("args[0] is not a cluster")
	}
	clusterCompDef, ok := args[1].(*appsv1alpha1.ClusterComponentDefinition)
	if !ok {
		return nil, nil, nil, nil, errors.New("args[1] not a cluster component definition")
	}
	clusterCompVer, ok := args[2].(*appsv1alpha1.ClusterComponentVersion)
	if !ok {
		return nil, nil, nil, nil, errors.New("args[2] not a cluster component version")
	}
	clusterCompSpec, ok := args[3].(*appsv1alpha1.ClusterComponentSpec)
	if !ok {
		return nil, nil, nil, nil, errors.New("args[3] not a cluster component spec")
	}
	return cluster, clusterCompDef, clusterCompVer, clusterCompSpec, nil
}

func (c *clusterConvertor) convert(args ...any) (any, error) {
	cluster, _, _, _, err := parseComponentConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	return cluster.Name, nil
}

func (c *compDefConvertor) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *classDefRefConvertor) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *serviceRefConvertor) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *resourceConvertor) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *volumeClaimTemplateConvertor) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *replicasConvertor) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *configConvertor2) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *monitorConvertor2) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *enabledLogConvertor) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

func (c *updateStrategyConvertor2) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *serviceAccountNameConvertor) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *affinityConvertor) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *tolerationConvertor) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *tlsConvertor) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}

func (c *issuerConvertor) convert(args ...any) (any, error) {
	// cluster, clusterCompDef, clusterCompVer, clusterCompSpec, err := parseComponentConvertorArgs(args...)
	return "", nil // TODO
}
