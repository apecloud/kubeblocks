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

package kafka

import (
	"context"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

const (
	publishTopic = "publishTopic"
	topics       = "topics"
)

type Manager struct {
	engines.DBManagerBase
	Kafka        *Kafka
	publishTopic string
	topics       []string
}

var _ engines.DBManager = &Manager{}

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	logger := ctrl.Log.WithName("Kafka")
	k := NewKafka(logger)
	// in kafka binding component, disable consumer retry by default
	k.DefaultConsumeRetryEnabled = false

	err := k.Init(context.TODO(), map[string]string(properties))
	if err != nil {
		return nil, err
	}

	managerBase, err := engines.NewDBManagerBase(logger)
	if err != nil {
		return nil, err
	}

	mgr := &Manager{
		DBManagerBase: *managerBase,
		Kafka:         k,
	}

	val, ok := properties[publishTopic]
	if ok && val != "" {
		mgr.publishTopic = val
	}

	val, ok = properties[topics]
	if ok && val != "" {
		mgr.topics = strings.Split(val, ",")
	}

	return mgr, nil
}

func (mgr *Manager) IsCurrentMemberHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	topic := "kb_health_check"

	err := mgr.Kafka.BrokerOpen()
	if err != nil {
		mgr.Logger.Info("broker open failed", "error", err)
		return false
	}
	defer mgr.Kafka.BrokerClose()

	err = mgr.Kafka.BrokerCreateTopics(topic)
	if err != nil {
		mgr.Logger.Info("create topic failed", "error", err)
		return false
	}
	return true
}
