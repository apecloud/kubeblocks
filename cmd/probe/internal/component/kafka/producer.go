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
	"errors"

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
	k.logger.Debugf("Publishing on topic %v", topic)

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

	k.logger.Debugf("Partition: %v, offset: %v", partition, offset)

	if err != nil {
		return err
	}

	return nil
}
