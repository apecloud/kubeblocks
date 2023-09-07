package apps

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/constant"
)

type CommonLabelAppender struct {
	client.Object
}

func (a *CommonLabelAppender) AddAppNameLabel(value string) *CommonLabelAppender {
	return a.addLabel(constant.AppNameLabelKey, value)
}

func (a *CommonLabelAppender) AddAppInstanceLabel(value string) *CommonLabelAppender {
	return a.addLabel(constant.AppInstanceLabelKey, value)
}

func (a *CommonLabelAppender) AddAppComponentLabel(value string) *CommonLabelAppender {
	return a.addLabel(constant.KBAppComponentLabelKey, value)
}

func (a *CommonLabelAppender) AddAppManagedByLabel() *CommonLabelAppender {
	return a.addLabel(constant.AppManagedByLabelKey, constant.AppName)
}

func (a *CommonLabelAppender) AddConsensusSetAccessModeLabel(value string) *CommonLabelAppender {
	return a.addLabel(constant.ConsensusSetAccessModeLabelKey, value)
}

func (a *CommonLabelAppender) AddRoleLabel(value string) *CommonLabelAppender {
	return a.addLabel(constant.RoleLabelKey, value)
}

func (a *CommonLabelAppender) GetObject() client.Object {
	return a.Object
}

func (a *CommonLabelAppender) addLabel(key, value string) *CommonLabelAppender {
	if a.Object == nil {
		return a
	}
	labels := a.Object.GetLabels()
	if labels == nil {
		labels = make(map[string]string, 0)
	}
	labels[key] = value
	a.Object.SetLabels(labels)
	return a
}
