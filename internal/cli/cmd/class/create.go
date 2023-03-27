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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type CreateOptions struct {
	genericclioptions.IOStreams

	Factory       cmdutil.Factory
	client        kubernetes.Interface
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
	if o.client, err = f.KubernetesClientSet(); err != nil {
		return err
	}
	if o.dynamic, err = f.DynamicClient(); err != nil {
		return err
	}
	return nil
}

func (o *CreateOptions) run() error {
	componentClasses, err := class.GetClasses(o.client, o.ClusterDefRef)
	if err != nil {
		return err
	}

	classes, ok := componentClasses[o.ComponentType]
	if !ok {
		classes = make(map[string]*class.ComponentClass)
	}

	families, err := class.GetClassFamilies(o.dynamic)
	if err != nil {
		return err
	}

	var (
		// new class definition version key
		cmK = class.BuildClassDefinitionVersion()
		// new class definition version value
		cmV string
		// newly created class names
		classNames []string
	)

	if o.File != "" {
		data, err := os.ReadFile(o.File)
		if err != nil {
			return err
		}
		newClasses, err := class.ParseComponentClasses(map[string]string{cmK: string(data)})
		if err != nil {
			return err
		}
		for name, cls := range newClasses {
			if _, ok = families[cls.Family]; !ok {
				return fmt.Errorf("family %s is not found", cls.Family)
			}
			if _, ok = classes[name]; ok {
				return fmt.Errorf("class name conflicted %s", name)
			}
			classNames = append(classNames, name)
		}
		cmV = string(data)
	} else {
		if _, ok = classes[o.ClassName]; ok {
			return fmt.Errorf("class name conflicted %s", o.ClassName)
		}
		if _, ok = families[o.ClassFamily]; !ok {
			return fmt.Errorf("family %s is not found", o.ClassFamily)
		}
		def, err := o.buildClassFamilyDef()
		if err != nil {
			return err
		}
		data, err := yaml.Marshal([]*class.ComponentClassFamilyDef{def})
		if err != nil {
			return err
		}
		cmV = string(data)
		classNames = append(classNames, o.ClassName)
	}

	cmName := class.GetCustomClassConfigMapName(o.ClusterDefRef, o.ComponentType)
	cm, err := o.client.CoreV1().ConfigMaps(CustomClassNamespace).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err == nil {
		cm.Data[cmK] = cmV
		if _, err = o.client.CoreV1().ConfigMaps(cm.GetNamespace()).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
			return err
		}
	} else {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      class.GetCustomClassConfigMapName(o.ClusterDefRef, o.ComponentType),
				Namespace: CustomClassNamespace,
				Labels: map[string]string{
					constant.ClusterDefLabelKey:     o.ClusterDefRef,
					types.ClassProviderLabelKey:     "user",
					types.ClassLevelLabelKey:        "component",
					constant.KBAppComponentLabelKey: o.ComponentType,
				},
			},
			Data: map[string]string{cmK: cmV},
		}
		if _, err = o.client.CoreV1().ConfigMaps(CustomClassNamespace).Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
			return err
		}
	}
	_, _ = fmt.Fprintf(o.Out, "Successfully created class [%s].", strings.Join(classNames, ","))
	return nil
}

func (o *CreateOptions) buildClassFamilyDef() (*class.ComponentClassFamilyDef, error) {
	clsDef := class.ComponentClassDef{Name: o.ClassName, CPU: o.CPU, Memory: o.Memory}
	for _, disk := range o.Storage {
		kvs := strings.Split(disk, ",")
		def := class.DiskDef{}
		for _, kv := range kvs {
			parts := strings.Split(kv, "=")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid storage disk: %s", disk)
			}
			switch parts[0] {
			case "name":
				def.Name = parts[1]
			case "size":
				def.Size = parts[1]
			case "class":
				def.Class = parts[1]
			default:
				return nil, fmt.Errorf("invalid storage disk: %s", disk)
			}
		}
		// validate disk size
		if _, err := resource.ParseQuantity(def.Size); err != nil {
			return nil, fmt.Errorf("invalid disk size: %s", disk)
		}
		if def.Name == "" {
			return nil, fmt.Errorf("invalid disk name: %s", disk)
		}
		clsDef.Storage = append(clsDef.Storage, def)
	}
	def := &class.ComponentClassFamilyDef{
		Family: o.ClassFamily,
		Series: []class.ComponentClassSeriesDef{{Classes: []class.ComponentClassDef{clsDef}}},
	}
	return def, nil
}
