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
	ClassFamily   string
	ComponentType string
	ClassName     string
	CPU           string
	Memory        string
	Storage       []string
	File          string
}

var classCreateExamples = templates.Examples(`
    # Create a class following class family kubeblocks-general-classes for component mysql in cluster definition apecloud-mysql, which have 1 cpu core, 2Gi memory and storage is 10Gi
    kbcli class create custom-1c2g --cluster-definition apecloud-mysql --type mysql --class-family kubeblocks-general-classes --cpu 1 --memory 2Gi --storage name=data,size=10Gi

    # Create classes for component mysql in cluster definition apecloud-mysql, where classes is defined in file
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
	cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", "", "Specify cluster definition, run \"kbcli cluster-definition list\" to show all available cluster definition")
	util.CheckErr(cmd.MarkFlagRequired("cluster-definition"))
	cmd.Flags().StringVar(&o.ComponentType, "type", "", "Specify component type")
	util.CheckErr(cmd.MarkFlagRequired("type"))

	cmd.Flags().StringVar(&o.ClassFamily, "class-family", "", "Specify class family")
	cmd.Flags().StringVar(&o.CPU, corev1.ResourceCPU.String(), "", "Specify component cpu cores")
	cmd.Flags().StringVar(&o.Memory, corev1.ResourceMemory.String(), "", "Specify component memory size")
	cmd.Flags().StringArrayVar(&o.Storage, corev1.ResourceStorage.String(), []string{}, "Specify component storage disks")

	cmd.Flags().StringVar(&o.File, "file", "", "Specify file path which contains YAML definition of class")

	return cmd
}

func (o *CreateOptions) validate(args []string) error {
	// just validate creating by resource arguments
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

	families, err := class.GetClassFamilies(o.dynamic)
	if err != nil {
		return err
	}

	var (
		classNames           []string
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
		for name, cls := range newClasses {
			if _, ok = families[cls.ClassConstraintRef]; !ok {
				return fmt.Errorf("family %s is not found", cls.ClassConstraintRef)
			}
			if _, ok = classes[name]; ok {
				return fmt.Errorf("class name conflicted %s", name)
			}
			classNames = append(classNames, name)
		}
	} else {
		if _, ok = classes[o.ClassName]; ok {
			return fmt.Errorf("class name conflicted %s", o.ClassName)
		}
		if _, ok = families[o.ClassFamily]; !ok {
			return fmt.Errorf("family %s is not found", o.ClassFamily)
		}
		cls, err := o.buildClass()
		if err != nil {
			return err
		}
		componentClassGroups = []v1alpha1.ComponentClassGroup{
			{
				ClassConstraintRef: o.ClassFamily,
				Series: []v1alpha1.ComponentClassSeries{
					{
						Classes: []v1alpha1.ComponentClass{*cls},
					},
				},
			},
		}
		classNames = append(classNames, o.ClassName)
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
		classDefinition = v1alpha1.ComponentClassDefinition{
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
	_, _ = fmt.Fprintf(o.Out, "Successfully created class [%s].", strings.Join(classNames, ","))
	return nil
}

func (o *CreateOptions) buildClass() (*v1alpha1.ComponentClass, error) {
	cls := v1alpha1.ComponentClass{Name: o.ClassName, CPU: o.CPU, Memory: o.Memory}
	for _, disk := range o.Storage {
		kvs := strings.Split(disk, ",")
		diskDef := v1alpha1.DiskDef{}
		for _, kv := range kvs {
			parts := strings.Split(kv, "=")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid storage disk: %s", disk)
			}
			switch parts[0] {
			case "name":
				diskDef.Name = parts[1]
			case "size":
				diskDef.Size = parts[1]
			case "class":
				diskDef.Class = parts[1]
			default:
				return nil, fmt.Errorf("invalid storage disk: %s", disk)
			}
		}
		// validate disk size
		if _, err := resource.ParseQuantity(diskDef.Size); err != nil {
			return nil, fmt.Errorf("invalid disk size: %s", disk)
		}
		if diskDef.Name == "" {
			return nil, fmt.Errorf("invalid disk name: %s", disk)
		}
		cls.Storage = append(cls.Storage, diskDef)
	}
	return &cls, nil
}
