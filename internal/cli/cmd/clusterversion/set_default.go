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

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var (
	setDefaultExample = templates.Examples(`
	# set ac-mysql-8.0.30 as the default clusterversion
	kbcli clusterversion set-default ac-mysql-8.0.30`,
	)

	unsetDefaultExample = templates.Examples(`
	# unset ac-mysql-8.0.30 from default clusterversion if it's default
	kbcli clusterversion unset-default ac-mysql-8.0.30`)

	clusterVersionGVR = types.ClusterVersionGVR()
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
		Short:             "Set the clusterversion to the default cluster clusterversion for its clusterdefinition.",
		Example:           setDefaultExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, clusterVersionGVR),
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
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, clusterVersionGVR),
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
	// unset-default logic
	if !o.toSetDefault {
		for _, cv := range args {
			if err := patchDefaultClusterVersionAnnotations(client, cv, annotationFalseValue); err != nil {
				allErrs = append(allErrs, err)
			}
		}
		return utilerrors.NewAggregate(allErrs)
	}
	// set-default logic
	cv2Cd, cd2DefaultCv, err := getAllClusterVersionAndDefault(client)
	if err != nil {
		return err
	}
	// alreadySet is to marks if two input args have the same clusterdefintion
	alreadySet := make(map[string]string)
	for _, cv := range args {
		if len(cv2Cd[cv]) == 0 {
			allErrs = append(allErrs, fmt.Errorf("clusterversion \"%s\" is not existed", cv))
			continue
		}
		if len(cd2DefaultCv[cv2Cd[cv]]) != 0 {
			allErrs = append(allErrs, fmt.Errorf("clusterdefinition \"%s\" already has a default cluster version \"%s\"", cv2Cd[cv], cd2DefaultCv[cv2Cd[cv]]))
			continue
		}
		if len(alreadySet[cv2Cd[cv]]) != 0 {
			allErrs = append(allErrs, fmt.Errorf("\"%s\" has the same clusterdefinition with \"%s\"", cv, alreadySet[cv2Cd[cv]]))
			continue
		}
		if err := patchDefaultClusterVersionAnnotations(client, cv, annotationTrueValue); err != nil {
			allErrs = append(allErrs, err)
		}
		alreadySet[cv2Cd[cv]] = cv
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
	_, err := client.Resource(clusterVersionGVR).Patch(context.Background(), cvName, apitypes.MergePatchType, patchBytes, metav1.PatchOptions{})
	return err
}

func getAllClusterVersionAndDefault(client dynamic.Interface) (map[string]string, map[string]string, error) {
	lists, err := client.Resource(clusterVersionGVR).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	cvToCd := make(map[string]string)
	cdToDefaultCv := make(map[string]string)
	for _, item := range lists.Items {
		name := item.GetName()
		annotations := item.GetAnnotations()
		labels := item.GetLabels()
		if labels == nil {
			// allErrs = append(allErrs, fmt.Errorf("cluterversion \"%s\" lacks of \"labels\" field"))
			continue
		}
		cvToCd[name] = labels[constant.ClusterDefLabelKey]
		if annotations == nil {
			// allErrs = append(allErrs, fmt.Errorf("cluterversion \"%s\" lacks of \"annotations\" field"))
			continue
		}
		if annotations[constant.DefaultClusterVersionAnnotationKey] == annotationTrueValue {
			cdToDefaultCv[labels[constant.ClusterDefLabelKey]] = name
		}
	}
	return cvToCd, cdToDefaultCv, nil
}
