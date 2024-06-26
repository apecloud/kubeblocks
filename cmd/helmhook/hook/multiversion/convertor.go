/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package multiversion

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/exp/maps"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
)

type sourceKind interface {
	list(ctx context.Context, cli *versioned.Clientset, namespace string) ([]client.Object, error)
}

type targetKind interface {
	kind() string
	get(ctx context.Context, cli *versioned.Clientset, namespace, name string) (client.Object, error)
	convert(source client.Object) client.Object
}

type convertor struct {
	namespaces []string
	sourceKind sourceKind
	targetKind targetKind

	stat    stat
	objects map[string]client.Object
}

func (c *convertor) Convert(ctx context.Context, cli hook.CRClient) ([]client.Object, error) {
	if len(c.namespaces) == 0 {
		c.namespaces = []string{""}
	}
	for _, namespace := range c.namespaces {
		sources, err := c.sourceKind.list(ctx, cli.KBClient, namespace)
		if err != nil {
			return nil, err
		}
		for _, obj := range sources {
			namespacedName := fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
			c.objects[namespacedName] = obj

			exist, converted, err1 := checkExistedNConverted(func() (client.Object, error) {
				return c.targetKind.get(ctx, cli.KBClient, obj.GetNamespace(), obj.GetName())
			})
			switch {
			case err1 != nil:
				c.stat.error(namespacedName, err1)
			case !exist:
				c.stat.convert(namespacedName)
			case converted:
				c.stat.converted(namespacedName)
			default:
				c.stat.native(namespacedName)
			}
		}
	}
	c.stat.dump(c.targetKind.kind())

	targets := make([]client.Object, 0)
	for name := range c.stat.toBeConverted {
		targets = append(targets, c.convert(c.objects[name]))
	}
	return targets, nil
}

func (c *convertor) convert(source client.Object) client.Object {
	target := c.targetKind.convert(source)

	// TODO: filter labels & annotations
	labels := func() map[string]string {
		return source.GetLabels()
	}
	annotations := func() map[string]string {
		m := map[string]string{}
		maps.Copy(m, source.GetAnnotations())
		b, _ := json.Marshal(source)
		m[convertedFromAnnotationKey] = string(b)
		return m
	}
	target.SetNamespace(source.GetNamespace())
	target.SetName(source.GetName())
	target.SetLabels(labels())
	target.SetAnnotations(annotations())
	return target
}
