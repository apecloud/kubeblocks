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

package configfsm

import (
	"context"
	"fmt"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/hsm"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ConfigStateType string

const (
	CInitPhase              ConfigStateType = "Init"
	CRenderingPhase         ConfigStateType = "Rendering"
	CGeneratingSidecarPhase ConfigStateType = "GeneratingSidecar"
	CCreatingConfigmapPhase ConfigStateType = "CreatingConfigmap"
	CRunningPhase           ConfigStateType = "Running"
	CInitFailedPhase        ConfigStateType = "InitFailed"
	CFailedPhase            ConfigStateType = "Failed"
	CRollbackPhase          ConfigStateType = "Rollback"
	CDeletingPhase          ConfigStateType = "Deleting"
	CFinishedPhase          ConfigStateType = "Finished"
)

const (
	CreatingOrUpdating = "upgrading"
	RenderedSucceed    = "rendered-succeed"
	RenderedFailed     = "rendered-failed"
	Reconfiguring      = "reconfiguring"
	ReRendering        = "rerendering"
)

const ConfigFSMID = "config-fsm"

var ConfigFSMSignature = func(_ ConfigStateType, _ string, _ ConfigFSMContext) {}

func (c ConfigStateType) OnEnter(ctx *ConfigFSMContext) error {
	log.FromContext(ctx.ctx).Info(fmt.Sprintf("entering state: %s", c))
	return nil
}

func (c ConfigStateType) OnExit(ctx *ConfigFSMContext) error {
	log.FromContext(ctx.ctx).Info(fmt.Sprintf("exiting state: %s", c))
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

// +--------------------------------------------------------------------------------------------------------------------------------------------------------------------+
// |                                                                                                                                                                    |
// |     ++---------------------------------------------------------------------------------------------------+                                                         |
// |     |+--------------------------------------------------------------------------------------------------+|                                                         |
// |     ||                                CInitPhase                                                        ||                                                         |
// |     ||                                                                                                  ||                                                         |
// |     ||   +-------------+          +---------------+            +-----------------+          +-----+     ||                                                         |
// |     ||   |             |          |               |            |                 |          |     |     ||  CreatingEvent                                          |
// |     ||   | PreparePhase|----------| RenderingPhase+----------->|GeneratingSideCar|--------->| END |     ||---------------- fireEvent                               |
// |     ||   |             |          |               |            |                 |          |     |     ||                    |                                    |
// |     ||   +-------------+          +---------------+            +-----------------+          +-----+     ||                    |                                    |
// |     ||                                                                                                  ||                    |                                    |
// |     ||                                                                                                  ||                    |                                    |
// |     ++--------------------------------------------------------------------------------------------------+|                    | ReconfiguringEvent                 |
// |     +----------------------------------------------------------------------------------------------------+                    |                                    |
// |           ^                    /                                           \                                                  |                                    |
// |           |                   /                                             \  Succeed                                        V                                    |
// |           | Creating         /                 +-----------------------------\----------------------------------------------------------------------------+        |
// |           |                 /                  |+-----------------------------V--------------------------------------------------------------------------+|        |
// |           |                /                   ||                                                                                                        ||        |
// |           |            Failed                  ||                                      RuningPhase                                                       ||        |
// |           |              /                     ||                                                              Reconfiguring                             ||        |
// |           |             /                      ||                                                   +---------------------------------+                  ||        |
// |           |            /                       ||                                                   v                                 |                  ||        |
// |           |           v                        ||      +---------------+                  +--------------------+             +--------+---------+        ||        |
// |      +----+----------------+                   ||      |               |  Reconfiguring   |                    |   Failed    |                  |        ||        |
// |      |                     |                   ||      |  CFinishPhase |----------------->| configMergingPhase |------------>| MergeFailedPhase |        ||        |
// |      | CCreateFailedPhase  |                   ||      |               |               -> |                    |             |                  |        ||        |
// |      |                     |                   ||      +-------+-------+              /   +---------+----------+             +------------------+        ||        |
// |      +---------------------+                   ||              ^                     /              |                                                    ||        |
// |                                                ||              |                Reconfiguring       |                                                    ||        |
// |                                                ||              |                   /                |                                                    ||        |
// |                                                ||       Finish |                  /                 | Succeed                                            ||        |
// |                                                ||              |                 /                  |                                                    ||        |
// |                                                ||              |     +-----------------+            |                                                    ||        |
// |                                                ||              |     |                 |            |                                                    ||        |
// |                                                ||              +-----+ UpgradingPhase  |<-----------+                                                    ||        |
// |                                                ||                    |                 |                                                                 ||        |
// |                                                ||                    +------------+----+                                                                 ||        |
// |                                                ||                         ^       |                                                                      ||        |
// |                                                ||                         |       |                                                                      ||        |
// |                                                ||                         +-------+                                                                      ||        |
// |                                                ||                                                                                                        ||        |
// |                                                ||                                                                                                        ||        |
// |                                                |+--------------------------------------------------------------------------------------------------------+|        |
// |                                                +----------------------------------------------------------------------------------------------------------+        |
// |                                                                                                                                                                    |
// |                                                                                                                                                                    |
// +--------------------------------------------------------------------------------------------------------------------------------------------------------------------+
//

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
	}).
		TemplateStateBuilder(CRenderingPhase, func(ctx *ConfigFSMContext) (hsm.State, error) {
			return CGeneratingSidecarPhase, createConfigmapResource(ctx)
		}).
		TemplateStateBuilder(CGeneratingSidecarPhase, func(ctx *ConfigFSMContext) (hsm.State, error) {
			return CCreatingConfigmapPhase, buildConfigManager(ctx)
		}).
		TemplateStateBuilder(CCreatingConfigmapPhase, func(ctx *ConfigFSMContext) (hsm.State, error) {
			return CRunningPhase, createConfigObjects(ctx.cli, ctx.ctx, ctx.renderWrapper.renderedObjs)
		})

	// construct the state machine
	sm.StateBuilder(CInitPhase).
		InternalTransition(CreatingOrUpdating, func(ctx *ConfigFSMContext) (hsm.State, error) {
			return CRenderingPhase, generateConfigurationResource(ctx)
		}).
		Build()
	sm.StateBuilder(CRunningPhase).
		Build()

	hsm.RegisterStateMachine(sm)
}
