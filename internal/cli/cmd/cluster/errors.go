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

package cluster

import (
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
)

var (
	clusterNotExistErrMessage          = "cluster[name=%s] does not exist. Please check that <cluster name> is spelled correctly."
	componentNotExistErrMessage        = "cluster[name=%s] does not have component[name=%s]. Please check that --component is spelled correctly."
	missingClusterArgErrMassage        = "cluster name should be specified, using --help."
	missingUpdatedParametersErrMessage = "missing updated parameters, using --help."

	multiComponentsErrorMessage     = "when multi components exist, specify a component, using --component"
	multiConfigTemplateErrorMessage = "when multi config templates exist, specify a config template,  using --config-spec"
	multiConfigFileErrorMessage     = "when multi config files exist, specify a config file, using --config-file"

	notFoundValidConfigTemplateErrorMessage = "cannot find valid config templates for component[name=%s] in the cluster[name=%s]"

	notFoundConfigSpecErrorMessage = "cannot find config spec[%s] for component[name=%s] in the cluster[name=%s]"

	notFoundConfigFileErrorMessage   = "cannot find config file[name=%s] in the configspec[name=%s], all configfiles: %v"
	notSupportFileUpdateErrorMessage = "not supported file[%s] for updating, current supported files: %v"

	notConfigSchemaPrompt         = "The config template[%s] is not defined in schema and parameter explanation info cannot be generated."
	cue2openAPISchemaFailedPrompt = "The cue schema may not satisfy the conversion constraints of openAPISchema and parameter explanation info cannot be generated."
	restartConfirmPrompt          = "The parameter change incurs a cluster restart, which brings the cluster down for a while. Enter to continue...\n, "
	fullRestartConfirmPrompt      = "The config file[%s] change incurs a cluster restart, which brings the cluster down for a while. Enter to continue...\n, "
	confirmApplyReconfigurePrompt = "Are you sure you want to apply these changes?\n"
)

func makeClusterNotExistErr(clusterName string) error {
	return cfgcore.MakeError(clusterNotExistErrMessage, clusterName)
}

func makeComponentNotExistErr(clusterName, component string) error {
	return cfgcore.MakeError(componentNotExistErrMessage, clusterName, component)
}

func makeConfigSpecNotExistErr(clusterName, component, configSpec string) error {
	return cfgcore.MakeError(notFoundConfigSpecErrorMessage, configSpec, component, clusterName)
}

func makeNotFoundTemplateErr(clusterName, component string) error {
	return cfgcore.MakeError(notFoundValidConfigTemplateErrorMessage, component, clusterName)
}

func makeNotFoundConfigFileErr(configFile, configSpec string, all []string) error {
	return cfgcore.MakeError(notFoundConfigFileErrorMessage, configFile, configSpec, all)
}

func makeNotSupportConfigFileUpdateErr(configFile string, configSpec appsv1alpha1.ComponentConfigSpec) error {
	return cfgcore.MakeError(notSupportFileUpdateErrorMessage, configFile, configSpec.Keys)
}

func makeMissingClusterNameErr() error {
	return cfgcore.MakeError(missingClusterArgErrMassage)
}
