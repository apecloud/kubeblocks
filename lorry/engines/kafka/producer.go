/*
Copyright 2021 The Dapr Authors
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
	"fmt"

	"github.com/Shopify/sarama"
)

func getSyncProducer(config sarama.Config, brokers []string, maxMessageBytes int) (sarama.SyncProducer, error) {
	// Add SyncProducer specific properties to copy of base config
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true

	if maxMessageBytes > 0 {
		config.Producer.MaxMessageBytes = maxMessageBytes
	}

	producer, err := sarama.NewSyncProducer(brokers, &config)
	if err != nil {
		return nil, err
	}

	return producer, nil
}

// Publish message to Kafka cluster.
func (k *Kafka) Publish(_ context.Context, topic string, data []byte, metadata map[string]string) error {
	if k.Producer == nil {
		return errors.New("component is closed")
	}
	// k.logger.Debugf("Publishing topic %v with data: %v", topic, string(data))
	k.logger.Info(fmt.Sprintf("Publishing on topic %v", topic))

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(data),
	}

	for name, value := range metadata {
		if name == key {
			msg.Key = sarama.StringEncoder(value)
		} else {
			if msg.Headers == nil {
				msg.Headers = make([]sarama.RecordHeader, 0, len(metadata))
			}
			msg.Headers = append(msg.Headers, sarama.RecordHeader{
				Key:   []byte(name),
				Value: []byte(value),
			})
		}
	}

	partition, offset, err := k.Producer.SendMessage(msg)

	k.logger.Info(fmt.Sprintf("Partition: %v, offset: %v", partition, offset))

	if err != nil {
		return err
	}

	return nil
}
