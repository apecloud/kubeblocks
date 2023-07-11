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

package configuration

import (
	"context"
	"fmt"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/hsm"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigStateType string

const (
	CInitPhase       ConfigStateType = "Init"
	CRunningPhase    ConfigStateType = "Running"
	CInitFailedPhase ConfigStateType = "InitFailed"
	CFailedPhase     ConfigStateType = "Failed"
	CRollbackPhase   ConfigStateType = "Rollback"
	CDeletingPhase   ConfigStateType = "Deleting"
	CFinishedPhase   ConfigStateType = "Finished"
)

const (
	Creating        = "creating"
	RenderedSucceed = "rendered-succeed"
	RenderedFailed  = "rendered-failed"
	Reconfiguring   = "reconfiguring"
	ReRendering     = "rerendering"
)

const ConfigFSMID = "config-fsm"

var ConfigFSMSignature = func(_ ConfigStateType, _ string, _ ConfigFSMContext) {}

func (c ConfigStateType) OnEnter(_ *ConfigFSMContext) error {
	return nil
}

func (c ConfigStateType) OnExit(_ *ConfigFSMContext) error {
	return nil
}

type ConfigFSMContext struct {
	hsm.BaseContext[ConfigStateType, ConfigFSMContext]

	cli       client.Client
	ctx       context.Context
	localObjs []client.Object

	clusterVersion *appsv1alpha1.ClusterVersion
	cluster        *appsv1alpha1.Cluster
	component      *component.SynthesizedComponent
	componentObj   client.Object
	podSpec        *corev1.PodSpec

	renderWrapper renderWrapper
}

func NewConfigContext(clusterVersion *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	obj client.Object,
	podSpec *corev1.PodSpec,
	localObjs []client.Object,
	ctx context.Context,
	cli client.Client) *ConfigFSMContext {
	context := &ConfigFSMContext{
		localObjs:      localObjs,
		ctx:            ctx,
		cli:            cli,
		cluster:        cluster,
		component:      component,
		componentObj:   obj,
		podSpec:        podSpec,
		clusterVersion: clusterVersion,
	}
	return context
}

func init() {
	sm := hsm.NewStateMachine(ConfigFSMID, CInitPhase, ConfigFSMSignature)
	sm.OnRecover(func(ctx *ConfigFSMContext) (ConfigStateType, error) {
		if len(ctx.component.ConfigTemplates) == 0 && len(ctx.component.ScriptTemplates) == 0 {
			return CFinishedPhase, nil
		}
		state := CRunningPhase
		if ok, err := isAllConfigmapReady(ctx); err != nil {
			return CRunningPhase, err
		} else if ok {
			state = CRunningPhase
		}
		return state, prepareConfigurationResource(ctx)
	})
	sm.StateBuilder(CInitPhase).
		Transition(RenderedSucceed, CRunningPhase).
		Transition(RenderedFailed, CInitFailedPhase).
		InternalTransition(Creating, func(ctx *ConfigFSMContext) error {
			return generateConfigurationResource(ctx)
		}).
		InternalTransition(Creating, func(ctx *ConfigFSMContext) error {
			return createConfigmapResource(ctx)
		}, func(ctx *ConfigFSMContext) bool {
			return checkConfigmapResource(ctx)
		}).
		InternalTransition(Creating, func(ctx *ConfigFSMContext) error {
			return buildConfigManager(ctx)
		}).
		InternalTransition(Creating, func(ctx *ConfigFSMContext) error {
			return createConfigObjects(ctx.cli, ctx.ctx, ctx.renderWrapper.renderedObjs)
		}).
		OnEnter(func(ctx *ConfigFSMContext) error {
			fmt.Printf("enter state: %s", ctx.GetState())
			return nil
		}).
		OnExit(func(ctx *ConfigFSMContext) error {
			fmt.Printf("exit state: %s", ctx.GetState())
			return nil
		}).
		Build()
	sm.StateBuilder(CRunningPhase).
		Transition(RenderedSucceed, CRunningPhase).
		Transition(RenderedFailed, CFailedPhase).
		Build()

	hsm.RegisterStateMachine(sm)
}
