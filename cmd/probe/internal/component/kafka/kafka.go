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
	"sync"
	"time"

	"github.com/Shopify/sarama"

	"github.com/dapr/kit/logger"
	"github.com/dapr/kit/retry"
)

// Kafka allows reading/writing to a Kafka consumer group.
type Kafka struct {
	Producer        sarama.SyncProducer
	broker          *sarama.Broker
	consumerGroup   string
	brokers         []string
	logger          logger.Logger
	authType        string
	saslUsername    string
	saslPassword    string
	initialOffset   int64
	cg              sarama.ConsumerGroup
	consumer        consumer
	config          *sarama.Config
	subscribeTopics TopicHandlerConfig
	subscribeLock   sync.Mutex

	backOffConfig retry.Config

	// The default value should be true for kafka pubsub component and false for kafka binding component
	// This default value can be overridden by metadata consumeRetryEnabled
	DefaultConsumeRetryEnabled bool
	consumeRetryEnabled        bool
	consumeRetryInterval       time.Duration
}

func NewKafka(logger logger.Logger) *Kafka {
	return &Kafka{
		logger:          logger,
		subscribeTopics: make(TopicHandlerConfig),
		subscribeLock:   sync.Mutex{},
	}
}

// Init does metadata parsing and connection establishment.
func (k *Kafka) Init(_ context.Context, metadata map[string]string) error {
	upgradedMetadata, err := k.upgradeMetadata(metadata)
	if err != nil {
		return err
	}

	meta, err := k.getKafkaMetadata(upgradedMetadata)
	if err != nil {
		return err
	}

	k.brokers = meta.Brokers
	k.consumerGroup = meta.ConsumerGroup
	k.initialOffset = meta.InitialOffset
	k.authType = meta.AuthType

	k.broker = sarama.NewBroker(k.brokers[0])

	config := sarama.NewConfig()
	config.Version = meta.Version
	config.Consumer.Offsets.Initial = k.initialOffset

	if meta.ClientID != "" {
		config.ClientID = meta.ClientID
	}

	err = updateTLSConfig(config, meta)
	if err != nil {
		return err
	}

	switch k.authType {
	case oidcAuthType:
		k.logger.Info("Configuring SASL OAuth2/OIDC authentication")
		err = updateOidcAuthInfo(config, meta)
		if err != nil {
			return err
		}
	case passwordAuthType:
		k.logger.Info("Configuring SASL Password authentication")
		k.saslUsername = meta.SaslUsername
		k.saslPassword = meta.SaslPassword
		updatePasswordAuthInfo(config, meta, k.saslUsername, k.saslPassword)
	case mtlsAuthType:
		k.logger.Info("Configuring mTLS authentcation")
		err = updateMTLSAuthInfo(config, meta)
		if err != nil {
			return err
		}
	}

	k.config = config
	sarama.Logger = SaramaLogBridge{daprLogger: k.logger}

	k.Producer, err = getSyncProducer(*k.config, k.brokers, meta.MaxMessageBytes)
	if err != nil {
		return err
	}

	// Default retry configuration is used if no
	// backOff properties are set.
	if err := retry.DecodeConfigWithPrefix(
		&k.backOffConfig,
		metadata,
		"backOff"); err != nil {
		return err
	}
	k.consumeRetryEnabled = meta.ConsumeRetryEnabled
	k.consumeRetryInterval = meta.ConsumeRetryInterval

	k.logger.Debug("Kafka message bus initialization complete")

	return nil
}

func (k *Kafka) Close() (err error) {
	k.closeSubscriptionResources()

	if k.Producer != nil {
		err = k.Producer.Close()
		k.Producer = nil
	}

	return err
}

// EventHandler is the handler used to handle the subscribed event.
type EventHandler func(ctx context.Context, msg *NewEvent) error

// BulkEventHandler is the handler used to handle the subscribed bulk event.
// type BulkEventHandler func(ctx context.Context, msg *KafkaBulkMessage) ([]pubsub.BulkSubscribeResponseEntry, error)

// SubscriptionHandlerConfig is the handler and configuration for subscription.
type SubscriptionHandlerConfig struct {
	IsBulkSubscribe bool
	Handler         EventHandler
}

// NewEvent is an event arriving from a message bus instance.
type NewEvent struct {
	Data        []byte            `json:"data"`
	Topic       string            `json:"topic"`
	Metadata    map[string]string `json:"metadata"`
	ContentType *string           `json:"contentType,omitempty"`
}

// KafkaBulkMessage is a bulk event arriving from a message bus instance.
type KafkaBulkMessage struct {
	Entries  []KafkaBulkMessageEntry `json:"entries"`
	Topic    string                  `json:"topic"`
	Metadata map[string]string       `json:"metadata"`
}

// KafkaBulkMessageEntry is an item contained inside bulk event arriving from a message bus instance.
type KafkaBulkMessageEntry struct {
	EntryID     string            `json:"entryId"` //nolint:stylecheck
	Event       []byte            `json:"event"`
	ContentType string            `json:"contentType,omitempty"`
	Metadata    map[string]string `json:"metadata"`
}

func (k *Kafka) BrokerOpen() error {
	connected, err := k.broker.Connected()
	if err != nil {
		k.logger.Info("broker connected err:%v", err)
		return err
	}
	if !connected {
		err = k.broker.Open(k.config)
		if err != nil {
			k.logger.Info("broker connected err:%v", err)
			return err
		}
	}

	return nil
}

func (k *Kafka) BrokerClose() {
	_ = k.broker.Close()
}

func (k *Kafka) BrokerCreateTopics(topic string) error {
	req := &sarama.CreateTopicsRequest{
		Version: 1,
		TopicDetails: map[string]*sarama.TopicDetail{
			topic: {
				NumPartitions:     -1,
				ReplicationFactor: -1,
			},
		},
		Timeout:      time.Second,
		ValidateOnly: false,
	}

	resp, err := k.broker.CreateTopics(req)
	if err != nil {
		k.logger.Infof("CheckStatus error: %v", err)
		return err
	} else {
		respErr := resp.TopicErrors[topic]
		// ErrNo details: https://cwiki.apache.org/confluence/display/KAFKA/A+Guide+To+The+Kafka+Protocol#AGuideToTheKafkaProtocol-ErrorCodes
		if respErr.Err != 0 {
			k.logger.Infof("CheckStatus error, errMsg: %s errNo: %d", respErr.Error(), int16(respErr.Err))
			return respErr
		}
		return nil
	}
}
