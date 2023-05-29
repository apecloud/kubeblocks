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
	"sync"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/kafka"
	. "github.com/apecloud/kubeblocks/cmd/probe/util"
)

const (
	publishTopic = "publishTopic"
	topics       = "topics"
)

type KafkaOperations struct {
	kafka        *kafka.Kafka
	publishTopic string
	topics       []string
	closeCh      chan struct{}
	mu           sync.Mutex
	BaseOperations
}

// NewKafka returns a new kafka binding instance.
func NewKafka(logger logger.Logger) bindings.OutputBinding {
	k := kafka.NewKafka(logger)
	// in kafka binding component, disable consumer retry by default
	k.DefaultConsumeRetryEnabled = false
	return &KafkaOperations{
		kafka:          k,
		closeCh:        make(chan struct{}),
		BaseOperations: BaseOperations{Logger: logger},
	}
}

func (kafkaOps *KafkaOperations) Init(metadata bindings.Metadata) error {
	kafkaOps.BaseOperations.Init(metadata)
	kafkaOps.Logger.Debug("Initializing kafka binding")
	kafkaOps.DBType = "kafka"
	kafkaOps.InitIfNeed = kafkaOps.initIfNeed
	// kafkaOps.BaseOperations.GetRole = kafkaOps.GetRole
	// kafkaOps.DBPort = kafkaOps.GetRunningPort()
	// kafkaOps.RegisterOperation(GetRoleOperation, kafkaOps.GetRoleOps)
	// kafkaOps.RegisterOperation(GetLagOperation, kafkaOps.GetLagOps)
	kafkaOps.RegisterOperation(CheckStatusOperation, kafkaOps.CheckStatusOps)
	// kafkaOps.RegisterOperation(ExecOperation, kafkaOps.ExecOps)
	// kafkaOps.RegisterOperation(QueryOperation, kafkaOps.QueryOps)
	return nil
}

func (kafkaOps *KafkaOperations) initIfNeed() bool {
	if kafkaOps.kafka.Producer == nil {
		go func() {
			err := kafkaOps.InitDelay()
			kafkaOps.Logger.Errorf("Kafka connection init failed: %v", err)
		}()
		return true
	}
	return false
}

func (kafkaOps *KafkaOperations) InitDelay() error {
	kafkaOps.mu.Lock()
	defer kafkaOps.mu.Unlock()
	if kafkaOps.kafka.Producer != nil {
		return nil
	}

	err := kafkaOps.kafka.Init(context.TODO(), kafkaOps.Metadata.Properties)
	if err != nil {
		return err
	}

	val, ok := kafkaOps.Metadata.Properties[publishTopic]
	if ok && val != "" {
		kafkaOps.publishTopic = val
	}

	val, ok = kafkaOps.Metadata.Properties[topics]
	if ok && val != "" {
		kafkaOps.topics = strings.Split(val, ",")
	}

	return nil
}

// CheckStatusOps design details: https://infracreate.feishu.cn/wiki/wikcndch7lMZJneMnRqaTvhQpwb#doxcnOUyQ4Mu0KiUo232dOr5aad
func (kafkaOps *KafkaOperations) CheckStatusOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	result := OpsResult{}
	topic := "kb_health_check"

	err := kafkaOps.kafka.BrokerOpen()
	if err != nil {
		result["event"] = OperationFailed
		result["message"] = err.Error()
		return result, nil
	}
	defer kafkaOps.kafka.BrokerClose()

	err = kafkaOps.kafka.BrokerCreateTopics(topic)
	if err != nil {
		result["event"] = OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = OperationSuccess
		result["message"] = "topic validateOnly success"
	}

	return result, nil
}
