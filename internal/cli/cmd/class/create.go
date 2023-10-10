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

package class

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type CreateOptions struct {
	genericclioptions.IOStreams

	// REVIEW: make this field a parameter which can be set by user
	objectName string

	Factory       cmdutil.Factory
	dynamic       dynamic.Interface
	ClusterDefRef string
	ComponentType string
	ClassName     string
	CPU           string
	Memory        string
	File          string
}

var classCreateExamples = templates.Examples(`
    # Create a class for component mysql in cluster definition apecloud-mysql, which has 1 CPU core and 1Gi memory
    kbcli class create custom-1c1g --cluster-definition apecloud-mysql --type mysql --cpu 1 --memory 1Gi

    # Create classes for component mysql in cluster definition apecloud-mysql, with classes defined in file
    kbcli class create --cluster-definition apecloud-mysql --type mysql --file ./classes.yaml
`)

func NewCreateCommand(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := CreateOptions{IOStreams: streams}
	cmd := &cobra.Command{
		Use:     "create [NAME]",
		Short:   "Create a class",
		Example: classCreateExamples,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f))
			util.CheckErr(o.validate(args))
			util.CheckErr(o.run())
		},
	}
	cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", "", "Specify cluster definition, run \"kbcli clusterdefinition list\" to show all available cluster definitions")
	util.CheckErr(cmd.MarkFlagRequired("cluster-definition"))
	cmd.Flags().StringVar(&o.ComponentType, "type", "", "Specify component type")
	util.CheckErr(cmd.MarkFlagRequired("type"))

	cmd.Flags().StringVar(&o.CPU, corev1.ResourceCPU.String(), "", "Specify component CPU cores")
	cmd.Flags().StringVar(&o.Memory, corev1.ResourceMemory.String(), "", "Specify component memory size")

	cmd.Flags().StringVar(&o.File, "file", "", "Specify file path of class definition YAML")

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func (o *CreateOptions) validate(args []string) error {
	// validate creating by resource arguments
	if o.File != "" {
		return nil
	}

	// validate cpu and memory
	if _, err := resource.ParseQuantity(o.CPU); err != nil {
		return err
	}
	if _, err := resource.ParseQuantity(o.Memory); err != nil {
		return err
	}

	// validate class name
	if len(args) == 0 {
		return fmt.Errorf("missing class name")
	}
	o.ClassName = args[0]

	return nil
}

func (o *CreateOptions) complete(f cmdutil.Factory) error {
	var err error
	o.dynamic, err = f.DynamicClient()
	return err
}

func (o *CreateOptions) run() error {
	clsMgr, err := class.GetManager(o.dynamic, o.ClusterDefRef)
	if err != nil {
		return err
	}

	constraints, err := class.GetResourceConstraints(o.dynamic)
	if err != nil {
		return err
	}

	var (
		classInstances       []*v1alpha1.ComponentClass
		componentClassGroups []v1alpha1.ComponentClassGroup
	)

	if o.File != "" {
		data, err := os.ReadFile(o.File)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(data, &componentClassGroups); err != nil {
			return err
		}
		classDefinition := v1alpha1.ComponentClassDefinition{
			Spec: v1alpha1.ComponentClassDefinitionSpec{Groups: componentClassGroups},
		}
		newClasses, err := class.ParseComponentClasses(classDefinition)
		if err != nil {
			return err
		}
		for _, cls := range newClasses {
			classInstances = append(classInstances, cls)
		}
	} else {
		cls := v1alpha1.ComponentClass{Name: o.ClassName, CPU: resource.MustParse(o.CPU), Memory: resource.MustParse(o.Memory)}
		if err != nil {
			return err
		}
		componentClassGroups = []v1alpha1.ComponentClassGroup{
			{
				Series: []v1alpha1.ComponentClassSeries{
					{
						Classes: []v1alpha1.ComponentClass{cls},
					},
				},
			},
		}
		classInstances = append(classInstances, &cls)
	}

	var (
		classNames []string
		objName    = o.objectName
	)
	if objName == "" {
		objName = class.GetCustomClassObjectName(o.ClusterDefRef, o.ComponentType)
	}

	var rules []v1alpha1.ResourceConstraintRule
	for _, constraint := range constraints {
		rules = append(rules, constraint.FindRules(o.ClusterDefRef, o.ComponentType)...)
	}

	for _, item := range classInstances {
		clsDefRef := v1alpha1.ClassDefRef{Name: objName, Class: item.Name}
		if clsMgr.HasClass(o.ComponentType, clsDefRef) {
			return fmt.Errorf("class name conflicted %s", item.Name)
		}
		classNames = append(classNames, item.Name)

		if len(rules) == 0 {
			continue
		}

		match := false
		for _, rule := range rules {
			if rule.ValidateResources(item.ToResourceRequirements().Requests) {
				match = true
				break
			}
		}
		if !match {
			return fmt.Errorf("class %s does not conform to its constraints", item.Name)
		}
	}

	obj, err := o.dynamic.Resource(types.ComponentClassDefinitionGVR()).Get(context.TODO(), objName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	var classDefinition v1alpha1.ComponentClassDefinition
	if err == nil {
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &classDefinition); err != nil {
			return err
		}
		classDefinition.Spec.Groups = append(classDefinition.Spec.Groups, componentClassGroups...)
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&classDefinition)
		if err != nil {
			return err
		}
		if _, err = o.dynamic.Resource(types.ComponentClassDefinitionGVR()).Update(
			context.Background(), &unstructured.Unstructured{Object: unstructuredMap}, metav1.UpdateOptions{}); err != nil {
			return err
		}
	} else {
		gvr := types.ComponentClassDefinitionGVR()
		classDefinition = v1alpha1.ComponentClassDefinition{
			TypeMeta: metav1.TypeMeta{
				Kind:       types.KindComponentClassDefinition,
				APIVersion: gvr.Group + "/" + gvr.Version,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: class.GetCustomClassObjectName(o.ClusterDefRef, o.ComponentType),
				Labels: map[string]string{
					constant.ClusterDefLabelKey:           o.ClusterDefRef,
					types.ClassProviderLabelKey:           "user",
					constant.KBAppComponentDefRefLabelKey: o.ComponentType,
				},
			},
			Spec: v1alpha1.ComponentClassDefinitionSpec{
				Groups: componentClassGroups,
			},
		}
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&classDefinition)
		if err != nil {
			return err
		}
		if _, err = o.dynamic.Resource(types.ComponentClassDefinitionGVR()).Create(
			context.Background(), &unstructured.Unstructured{Object: unstructuredMap}, metav1.CreateOptions{}); err != nil {
			return err
		}
	}
	_, _ = fmt.Fprintf(o.Out, "Successfully create class [%s].\n", strings.Join(classNames, ","))
	return nil
}

func registerFlagCompletionFunc(cmd *cobra.Command, f cmdutil.Factory) {
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cluster-definition",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.CompGetResource(f, util.GVRToString(types.ClusterDefGVR()), toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"type",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var (
				componentTypes []string
				selector       string
				compTypeLabel  = "apps.kubeblocks.io/component-def-ref"
			)

			client, err := f.DynamicClient()
			if err != nil {
				return componentTypes, cobra.ShellCompDirectiveNoFileComp
			}

			clusterDefinition, err := cmd.Flags().GetString("cluster-definition")
			if err == nil && clusterDefinition != "" {
				selector = fmt.Sprintf("%s=%s,%s", constant.ClusterDefLabelKey, clusterDefinition, types.ClassProviderLabelKey)
			}
			objs, err := client.Resource(types.ComponentClassDefinitionGVR()).List(context.Background(), metav1.ListOptions{LabelSelector: selector})
			if err != nil {
				return componentTypes, cobra.ShellCompDirectiveNoFileComp
			}
			var classDefinitionList v1alpha1.ComponentClassDefinitionList
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(objs.UnstructuredContent(), &classDefinitionList); err != nil {
				return componentTypes, cobra.ShellCompDirectiveNoFileComp
			}
			for _, item := range classDefinitionList.Items {
				componentType := item.Labels[compTypeLabel]
				if componentType != "" {
					componentTypes = append(componentTypes, componentType)
				}
			}
			return componentTypes, cobra.ShellCompDirectiveNoFileComp
		}))
}
