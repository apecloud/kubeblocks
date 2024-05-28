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

package hook

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Conversion struct {
	BasedHandler
}

type ConversionHandler interface {
	Convert(context.Context, CRClient) ([]client.Object, error)
}

type ConversionMeta struct {
	ConversionHandler
	FromVersion []Version
	ToVersion   Version
}

var crConversionMaps = map[schema.GroupVersionResource]ConversionMeta{}

func (p *Conversion) Handle(ctx *UpgradeContext) (err error) {
	var (
		fromVersion = ctx.From
		toVersion   = ctx.To
	)

	for gvr, meta := range crConversionMaps {
		if !matchVersion(meta, *fromVersion, toVersion) || meta.ConversionHandler == nil {
			Log("no gvr to convert and pass, oldVersion: %v, newVersion: %v", fromVersion, toVersion)
			continue
		}
		objs, err := meta.Convert(ctx, ctx.CRClient)
		if err != nil {
			return err
		}
		if len(objs) != 0 {
			ctx.UpdatedObjects[gvr] = append(ctx.UpdatedObjects[gvr], objs...)
		}
	}
	return nil
}

func matchVersion(meta ConversionMeta, oldVersion Version, newVersion Version) bool {
	if meta.ToVersion != newVersion {
		return false
	}
	for _, version := range meta.FromVersion {
		if version == oldVersion {
			return true
		}
	}
	return true
}

func RegisterCRDConversion(gvr schema.GroupVersionResource, newVersion Version, handler ConversionHandler, oldVersions ...Version) {
	crConversionMaps[gvr] = ConversionMeta{
		FromVersion:       oldVersions,
		ToVersion:         newVersion,
		ConversionHandler: handler,
	}
}
