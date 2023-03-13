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

package extensions

const (
	// name of our custom finalizer
	addonFinalizerName = "addon.kubeblocks.io/finalizer"

	// annotation keys
	ControllerPaused     = "controller.kubeblocks.io/controller-paused"
	SkipInstallableCheck = "extensions.kubeblocks.io/skip-installable-check"
	NoDeleteJobs         = "extensions.kubeblocks.io/no-delete-jobs"

	// condition reasons
	AddonDisabled                    = "AddonDisabled"
	AddonEnabled                     = "AddonEnabled"
	AddonSpecInstallFailed           = "AddonSpecInstallFailed"
	AddonSpecInstallableReqUnmatched = "AddonSpecInstallableRequirementUnmatched"

	// event reasons
	InstallableCheckSkipped         = "InstallableCheckSkipped"
	InstallableRequirementUnmatched = "InstallableRequirementUnmatched"
	AddonAutoInstall                = "AddonAutoInstall"
	DisablingAddon                  = "DisablingAddon"
	EnablingAddon                   = "EnablingAddon"
	InstallationFailed              = "InstallationFailed"
	UninstallationFailed            = "UninstallationFailed"

	// config keys used in viper
	maxConcurrentReconcilesKey = "MAXCONCURRENTRECONCILES_ADDON"
	addonSANameKey             = "KUBEBLOCKS_ADDON_SA_NAME"
)
