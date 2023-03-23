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

package kafka

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/kafka"
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
	closed       atomic.Bool
	wg           sync.WaitGroup
	mu           sync.Mutex
	BaseOperations
}

// NewKafka returns a new kafka binding instance.
func NewKafka(logger logger.Logger) bindings.InputOutputBinding {
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
	// kafkaOps.RegisterOperation(CheckStatusOperation, kafkaOps.CheckStatusOps)
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

func (kafkaOps *KafkaOperations) Operations() []bindings.OperationKind {
	return []bindings.OperationKind{bindings.CreateOperation}
}

func (kafkaOps *KafkaOperations) Close() (err error) {
	if kafkaOps.closed.CompareAndSwap(false, true) {
		close(kafkaOps.closeCh)
	}
	defer kafkaOps.wg.Wait()
	return kafkaOps.kafka.Close()
}

func (kafkaOps *KafkaOperations) Invoke(ctx context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	err := kafkaOps.kafka.Publish(ctx, kafkaOps.publishTopic, req.Data, req.Metadata)
	return nil, err
}

func (kafkaOps *KafkaOperations) Read(ctx context.Context, handler bindings.Handler) error {
	if kafkaOps.closed.Load() {
		return errors.New("error: binding is closed")
	}

	if len(kafkaOps.topics) == 0 {
		kafkaOps.Logger.Warnf("kafka binding: no topic defined, input bindings will not be started")
		return nil
	}

	handlerConfig := kafka.SubscriptionHandlerConfig{
		IsBulkSubscribe: false,
		Handler:         adaptHandler(handler),
	}
	for _, t := range kafkaOps.topics {
		kafkaOps.kafka.AddTopicHandler(t, handlerConfig)
	}
	kafkaOps.wg.Add(1)
	go func() {
		defer kafkaOps.wg.Done()
		// Wait for context cancelation or closure.
		select {
		case <-ctx.Done():
		case <-kafkaOps.closeCh:
		}

		// Remove the topic handlers.
		for _, t := range kafkaOps.topics {
			kafkaOps.kafka.RemoveTopicHandler(t)
		}
	}()

	return kafkaOps.kafka.Subscribe(ctx)
}

func adaptHandler(handler bindings.Handler) kafka.EventHandler {
	return func(ctx context.Context, event *kafka.NewEvent) error {
		_, err := handler(ctx, &bindings.ReadResponse{
			Data:        event.Data,
			Metadata:    event.Metadata,
			ContentType: event.ContentType,
		})
		return err
	}
}
