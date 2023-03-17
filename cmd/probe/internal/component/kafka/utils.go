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
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/Shopify/sarama"
)

const (
	// DefaultMaxBulkSubCount is the default max bulk count for kafka pubsub component
	// if the MaxBulkCountKey is not set in the metadata.
	DefaultMaxBulkSubCount = 80
	// DefaultMaxBulkSubAwaitDurationMs is the default max bulk await duration for kafka pubsub component
	// if the MaxBulkAwaitDurationKey is not set in the metadata.
	DefaultMaxBulkSubAwaitDurationMs = 10000
)

// asBase64String implements the `fmt.Stringer` interface in order to print
// `[]byte` as a base 64 encoded string.
// It is used above to log the message key. The call to `EncodeToString`
// only occurs for logs that are written based on the logging level.
type asBase64String []byte

func (s asBase64String) String() string {
	return base64.StdEncoding.EncodeToString(s)
}

func parseInitialOffset(value string) (initialOffset int64, err error) {
	initialOffset = sarama.OffsetNewest // Default
	if strings.EqualFold(value, "oldest") {
		initialOffset = sarama.OffsetOldest
	} else if strings.EqualFold(value, "newest") {
		initialOffset = sarama.OffsetNewest
	} else if value != "" {
		return 0, fmt.Errorf("kafka error: invalid initialOffset: %s", value)
	}

	return initialOffset, err
}

// isValidPEM validates the provided input has PEM formatted block.
func isValidPEM(val string) bool {
	block, _ := pem.Decode([]byte(val))

	return block != nil
}

// TopicHandlerConfig is the map of topics and sruct containing handler and their config.
type TopicHandlerConfig map[string]SubscriptionHandlerConfig

// // TopicList returns the list of topics
func (tbh TopicHandlerConfig) TopicList() []string {
	topics := make([]string, len(tbh))
	i := 0
	for topic := range tbh {
		topics[i] = topic
		i++
	}
	return topics
}
