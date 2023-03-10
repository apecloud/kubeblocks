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

package cluster

import cfgcore "github.com/apecloud/kubeblocks/internal/configuration"

var (
	clusterNotExistErrMessage          = "cluster[name=%s] is not exist. Please check that <cluster name> is spelled correctly."
	componentNotExistErrMessage        = "cluster[name=%s] does not has this component[name=%s]. Please check that --component-name is spelled correctly."
	missingClusterArgErrMassage        = "cluster name should be specified, using --help."
	missingUpdatedParametersErrMessage = "missing updated parameters, using --help."

	multiComponentsErrorMessage     = "when multi component exist, must specify which component to use. Please using --component-name"
	multiConfigTemplateErrorMessage = "when multi config template exist, must specify which config template to use. Please using --template-name"

	notFoundValidConfigTemplateErrorMessage = "not find valid config template, component[name=%s] in the cluster[name=%s]"

	notCueSchemaPrompt            = "The config template not define cue schema and parameter explain info cannot be generated."
	cue2openAPISchemaFailedPrompt = "The cue schema may not satisfy the conversion constraints of openAPISchema and parameter explain info cannot be generated."
)

func makeClusterNotExistErr(clusterName string) error {
	return cfgcore.MakeError(clusterNotExistErrMessage, clusterName)
}

func makeComponentNotExistErr(clusterName, component string) error {
	return cfgcore.MakeError(componentNotExistErrMessage, clusterName, component)
}

func makeNotFoundTemplateErr(clusterName, component string) error {
	return cfgcore.MakeError(notFoundValidConfigTemplateErrorMessage, clusterName, component)
}

func makeMissingClusterNameErr() error {
	return cfgcore.MakeError(missingClusterArgErrMassage)
}
