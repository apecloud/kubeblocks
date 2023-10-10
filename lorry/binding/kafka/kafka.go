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

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	. "github.com/apecloud/kubeblocks/lorry/binding"
	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/component/kafka"
	"github.com/apecloud/kubeblocks/lorry/util"
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
func NewKafka() *KafkaOperations {
	logger := ctrl.Log.WithName("Kafka")
	k := kafka.NewKafka(logger)
	// in kafka binding component, disable consumer retry by default
	k.DefaultConsumeRetryEnabled = false
	return &KafkaOperations{
		kafka:          k,
		closeCh:        make(chan struct{}),
		BaseOperations: BaseOperations{Logger: logger},
	}
}

func (kafkaOps *KafkaOperations) Init(metadata component.Properties) error {
	kafkaOps.Logger.Info("Initializing kafka binding")
	kafkaOps.BaseOperations.Init(metadata)
	kafkaOps.Logger.Info("Initializing kafka binding")
	kafkaOps.DBType = "kafka"
	kafkaOps.InitIfNeed = kafkaOps.initIfNeed
	// kafkaOps.BaseOperations.GetRole = kafkaOps.GetRole
	// kafkaOps.DBPort = kafkaOps.GetRunningPort()
	// kafkaOps.RegisterOperation(GetRoleOperation, kafkaOps.GetRoleOps)
	// kafkaOps.RegisterOperation(GetLagOperation, kafkaOps.GetLagOps)
	kafkaOps.RegisterOperation(util.CheckStatusOperation, kafkaOps.CheckStatusOps)
	// kafkaOps.RegisterOperation(ExecOperation, kafkaOps.ExecOps)
	// kafkaOps.RegisterOperation(QueryOperation, kafkaOps.QueryOps)
	return nil
}

func (kafkaOps *KafkaOperations) initIfNeed() bool {
	if kafkaOps.kafka.Producer == nil {
		go func() {
			err := kafkaOps.InitDelay()
			if err != nil {
				kafkaOps.Logger.Error(err, "Kafka connection init failed")
			}
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

	err := kafkaOps.kafka.Init(context.TODO(), kafkaOps.Metadata)
	if err != nil {
		return err
	}

	val, ok := kafkaOps.Metadata[publishTopic]
	if ok && val != "" {
		kafkaOps.publishTopic = val
	}

	val, ok = kafkaOps.Metadata[topics]
	if ok && val != "" {
		kafkaOps.topics = strings.Split(val, ",")
	}

	return nil
}

func (kafkaOps *KafkaOperations) CheckStatusOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	result := OpsResult{}
	topic := "kb_health_check"

	err := kafkaOps.kafka.BrokerOpen()
	if err != nil {
		result["event"] = util.OperationFailed
		result["message"] = err.Error()
		return result, nil
	}
	defer kafkaOps.kafka.BrokerClose()

	err = kafkaOps.kafka.BrokerCreateTopics(topic)
	if err != nil {
		result["event"] = util.OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = util.OperationSuccess
		result["message"] = "topic validateOnly success"
	}

	return result, nil
}

func (kafkaOps *KafkaOperations) InternalQuery(ctx context.Context, sql string) ([]byte, error) {
	// TODO: impl
	return nil, nil
}

func (kafkaOps *KafkaOperations) InternalExec(ctx context.Context, sql string) (int64, error) {
	// TODO: impl
	return 0, nil
}

func (kafkaOps *KafkaOperations) GetLogger() logr.Logger {
	return kafkaOps.Logger
}

func (kafkaOps *KafkaOperations) GetRunningPort() int {
	// TODO: impl
	return kafkaOps.DBPort
}
