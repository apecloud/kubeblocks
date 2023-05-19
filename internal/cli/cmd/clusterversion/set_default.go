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

package clusterversion

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var (
	setDefaultExample = templates.Examples(`
	# set the clusterversion to be the default
	kbcli clusterversion set-default ac-mysql-8.0.30`,
	)

	unsetDefaultExample = templates.Examples(`
	# set the clusterversion not to be the default if it's default
	kbcli clusterversion unset-default ac-mysql-8.0.30`)

	ClusterVersionGVR    = types.ClusterVersionGVR()
	ClusterDefinitionGVR = types.ClusterDefGVR()
)

const (
	annotationTrueValue  = "true"
	annotationFalseValue = "false"
)

type SetOrUnsetDefaultOption struct {
	Factory   cmdutil.Factory
	IOStreams genericclioptions.IOStreams
	// `set-default` cmd will set the toSetDefault to be true, while `unset-default` cmd set it false
	toSetDefault bool
}

func newSetOrUnsetDefaultOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, toSet bool) *SetOrUnsetDefaultOption {
	return &SetOrUnsetDefaultOption{
		Factory:      f,
		IOStreams:    streams,
		toSetDefault: toSet,
	}
}

func newSetDefaultCMD(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newSetOrUnsetDefaultOptions(f, streams, true)
	cmd := &cobra.Command{
		Use:               "set-default NAME",
		Short:             "Set the clusterversion to the default cluster clusterversion for its clusterDef.",
		Example:           setDefaultExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, ClusterVersionGVR),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.validate(args))
			util.CheckErr(o.run(args))
		},
	}
	return cmd
}

func newUnSetDefaultCMD(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newSetOrUnsetDefaultOptions(f, streams, false)
	cmd := &cobra.Command{
		Use:               "unset-default NAME",
		Short:             "Unset the clusterversion if it's default.",
		Example:           unsetDefaultExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, ClusterVersionGVR),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.validate(args))
			util.CheckErr(o.run(args))
		},
	}
	return cmd
}

func (o *SetOrUnsetDefaultOption) run(args []string) error {
	client, err := o.Factory.DynamicClient()
	if err != nil {
		return err
	}
	var allErrs []error
	// unset-default logic:
	if !o.toSetDefault {
		for _, cv := range args {
			if err := patchDefaultClusterVersionAnnotations(client, cv, annotationFalseValue); err != nil {
				allErrs = append(allErrs, err)
			}
		}
		return utilerrors.NewAggregate(allErrs)
	}
	// set-default logic
	allClusterDefinition := make(map[string]string)
	// find all clusterDefinitionRef of the input cv
	for _, cv := range args {
		item, err := client.Resource(ClusterVersionGVR).Get(context.Background(), cv, metav1.GetOptions{})
		if err != nil {
			allErrs = append(allErrs, err)
			continue
		}
		labels := item.GetLabels()
		if labels == nil {
			// Absolutely not expected to happen， if return this error, the clusterversion has a wrong structure that without labels
			allErrs = append(allErrs, fmt.Errorf("the \"%s\" lacks of labels", cv))
			continue
		}
		// if labels don't have the KEY, the error will throw late, so we don't throw here
		cd := labels[constant.ClusterDefLabelKey]
		if _, ok := allClusterDefinition[cd]; ok {
			// the args belong the same clusterDefinitionRef
			allErrs = append(allErrs, fmt.Errorf("\"%s\" has the same clusterDef with \"%s\"", cv, allClusterDefinition[cd]))
			continue
		}
		allClusterDefinition[cd] = cv
	}

	for cd := range allClusterDefinition {
		versions, defaultcv, existedDefault, err := cluster.GetClusterVersions(client, cd)
		cv := allClusterDefinition[cd]
		if err != nil {
			allErrs = append(allErrs, err)
			continue
		}
		if _, ok := versions[cv]; !ok {
			// Absolutely not expected to happen， if return this error we must check the clusterdefinition we have created
			allErrs = append(allErrs, fmt.Errorf("clusterDefinition \"%s\" don't have a clusterversion \"%s\"", cd, cv))
			continue
		}
		if existedDefault {
			// The clusterDef already has a default cluster version
			allErrs = append(allErrs, fmt.Errorf("clusterDefinition %s already has a default cluster version \"%s\"", cd, defaultcv))
			continue
		}
		if err := patchDefaultClusterVersionAnnotations(client, cv, annotationTrueValue); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	return utilerrors.NewAggregate(allErrs)
}

func (o *SetOrUnsetDefaultOption) validate(args []string) error {
	if len(args) == 0 {
		if o.toSetDefault {
			return fmt.Errorf("set-default shuold specify the clusterversions. use \"kbcli clusterversion list\" to list the clusterversions")
		} else {
			return fmt.Errorf("unset-default shuold specify the clusterversions. use \"kbcli clusterversion list\" to list the clusterversions")
		}
	}
	return nil
}

// patchDefaultClusterVersionAnnotations will patch the Annotations for the clusterversion in K8S
func patchDefaultClusterVersionAnnotations(client dynamic.Interface, cvName string, value string) error {
	patchData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				constant.DefaultClusterVersionAnnotationKey: value,
			},
		},
	}
	patchBytes, _ := json.Marshal(patchData)
	_, err := client.Resource(ClusterVersionGVR).Patch(context.Background(), cvName, apitypes.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return err
	}
	return nil
}
