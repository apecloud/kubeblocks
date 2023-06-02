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
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type CreateOptions struct {
	genericclioptions.IOStreams

	Factory       cmdutil.Factory
	dynamic       dynamic.Interface
	ClusterDefRef string
	Constraint    string
	ComponentType string
	ClassName     string
	CPU           string
	Memory        string
	File          string
}

var classCreateExamples = templates.Examples(`
    # Create a class with constraint kb-resource-constraint-general for component mysql in cluster definition apecloud-mysql, which has 1 CPU core and 1Gi memory
    kbcli class create custom-1c1g --cluster-definition apecloud-mysql --type mysql --constraint kb-resource-constraint-general --cpu 1 --memory 1Gi

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

	cmd.Flags().StringVar(&o.Constraint, "constraint", "", "Specify resource constraint")
	cmd.Flags().StringVar(&o.CPU, corev1.ResourceCPU.String(), "", "Specify component CPU cores")
	cmd.Flags().StringVar(&o.Memory, corev1.ResourceMemory.String(), "", "Specify component memory size")

	cmd.Flags().StringVar(&o.File, "file", "", "Specify file path of class definition YAML")

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
	componentClasses, err := class.ListClassesByClusterDefinition(o.dynamic, o.ClusterDefRef)
	if err != nil {
		return err
	}

	classes, ok := componentClasses[o.ComponentType]
	if !ok {
		classes = make(map[string]*v1alpha1.ComponentClassInstance)
	}

	constraints, err := class.GetResourceConstraints(o.dynamic)
	if err != nil {
		return err
	}

	var (
		classInstances       []*v1alpha1.ComponentClassInstance
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
		if _, ok = classes[o.ClassName]; ok {
			return fmt.Errorf("class name conflicted %s", o.ClassName)
		}
		if _, ok = constraints[o.Constraint]; !ok {
			return fmt.Errorf("resource constraint %s is not found", o.Constraint)
		}
		cls := v1alpha1.ComponentClass{Name: o.ClassName, CPU: resource.MustParse(o.CPU), Memory: resource.MustParse(o.Memory)}
		if err != nil {
			return err
		}
		componentClassGroups = []v1alpha1.ComponentClassGroup{
			{
				ResourceConstraintRef: o.Constraint,
				Series: []v1alpha1.ComponentClassSeries{
					{
						Classes: []v1alpha1.ComponentClass{cls},
					},
				},
			},
		}
		classInstances = append(classInstances, &v1alpha1.ComponentClassInstance{ComponentClass: cls, ResourceConstraintRef: o.Constraint})
	}

	var classNames []string
	for _, item := range classInstances {
		constraint, ok := constraints[item.ResourceConstraintRef]
		if !ok {
			return fmt.Errorf("resource constraint %s is not found", item.ResourceConstraintRef)
		}
		if _, ok = classes[item.Name]; ok {
			return fmt.Errorf("class name conflicted %s", item.Name)
		}
		if !constraint.MatchClass(item) {
			return fmt.Errorf("class %s does not conform to constraint %s", item.Name, item.ResourceConstraintRef)
		}
		classNames = append(classNames, item.Name)
	}

	objName := class.GetCustomClassObjectName(o.ClusterDefRef, o.ComponentType)
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
