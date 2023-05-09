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

package extensions

const (
	// name of our custom finalizer
	addonFinalizerName = "addon.kubeblocks.io/finalizer"

	// annotation keys
	ControllerPaused     = "controller.kubeblocks.io/controller-paused"
	SkipInstallableCheck = "extensions.kubeblocks.io/skip-installable-check"
	NoDeleteJobs         = "extensions.kubeblocks.io/no-delete-jobs"
	AddonDefaultIsEmpty  = "addons.extensions.kubeblocks.io/default-is-empty"

	// condition reasons
	AddonDisabled = "AddonDisabled"
	AddonEnabled  = "AddonEnabled"

	// event reasons
	InstallableCheckSkipped         = "InstallableCheckSkipped"
	InstallableRequirementUnmatched = "InstallableRequirementUnmatched"
	AddonAutoInstall                = "AddonAutoInstall"
	AddonSetDefaultValues           = "AddonSetDefaultValues"
	DisablingAddon                  = "DisablingAddon"
	EnablingAddon                   = "EnablingAddon"
	InstallationFailed              = "InstallationFailed"
	InstallationFailedLogs          = "InstallationFailedLogs"
	UninstallationFailed            = "UninstallationFailed"
	UninstallationFailedLogs        = "UninstallationFailedLogs"
	AddonRefObjError                = "ReferenceObjectError"

	// config keys used in viper
	maxConcurrentReconcilesKey = "MAXCONCURRENTRECONCILES_ADDON"
	addonSANameKey             = "KUBEBLOCKS_ADDON_SA_NAME"
	addonHelmInstallOptKey     = "KUBEBLOCKS_ADDON_HELM_INSTALL_OPTIONS"
	addonHelmUninstallOptKey   = "KUBEBLOCKS_ADDON_HELM_UNINSTALL_OPTIONS"
)
