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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Shopify/sarama"
)

const (
	key                  = "partitionKey"
	skipVerify           = "skipVerify"
	caCert               = "caCert"
	clientCert           = "clientCert"
	clientKey            = "clientKey"
	consumeRetryEnabled  = "consumeRetryEnabled"
	consumeRetryInterval = "consumeRetryInterval"
	authType             = "authType"
	passwordAuthType     = "password"
	oidcAuthType         = "oidc"
	mtlsAuthType         = "mtls"
	noAuthType           = "none"
)

type kafkaMetadata struct {
	Brokers              []string
	ConsumerGroup        string
	ClientID             string
	AuthType             string
	SaslUsername         string
	SaslPassword         string
	SaslMechanism        string
	InitialOffset        int64
	MaxMessageBytes      int
	OidcTokenEndpoint    string
	OidcClientID         string
	OidcClientSecret     string
	OidcScopes           []string
	TLSDisable           bool
	TLSSkipVerify        bool
	TLSCaCert            string
	TLSClientCert        string
	TLSClientKey         string
	ConsumeRetryEnabled  bool
	ConsumeRetryInterval time.Duration
	Version              sarama.KafkaVersion
}

// upgradeMetadata updates metadata properties based on deprecated usage.
func (k *Kafka) upgradeMetadata(metadata map[string]string) (map[string]string, error) {
	authTypeVal, authTypePres := metadata[authType]
	authReqVal, authReqPres := metadata["authRequired"]
	saslPassVal, saslPassPres := metadata["saslPassword"]

	// If authType is not set, derive it from authRequired.
	if (!authTypePres || authTypeVal == "") && authReqPres && authReqVal != "" {
		k.logger.Warn("AuthRequired is deprecated, use AuthType instead.")
		validAuthRequired, err := strconv.ParseBool(authReqVal)
		if err == nil {
			if validAuthRequired {
				// If legacy authRequired was used, either SASL username or mtls is the method.
				if saslPassPres && saslPassVal != "" {
					// User has specified saslPassword, so intend for password auth.
					metadata[authType] = passwordAuthType
				} else {
					metadata[authType] = mtlsAuthType
				}
			} else {
				metadata[authType] = noAuthType
			}
		} else {
			return metadata, errors.New("kafka error: invalid value for 'authRequired' attribute")
		}
	}

	// if consumeRetryEnabled is not present, use component default value
	consumeRetryEnabledVal, consumeRetryEnabledPres := metadata[consumeRetryEnabled]
	if !consumeRetryEnabledPres || consumeRetryEnabledVal == "" {
		metadata[consumeRetryEnabled] = strconv.FormatBool(k.DefaultConsumeRetryEnabled)
	}

	return metadata, nil
}

// getKafkaMetadata returns new Kafka metadata.
func (k *Kafka) getKafkaMetadata(metadata map[string]string) (*kafkaMetadata, error) {
	meta := kafkaMetadata{
		ConsumeRetryInterval: 100 * time.Millisecond,
	}
	// use the runtimeConfig.ID as the consumer group so that each dapr runtime creates its own consumergroup
	if val, ok := metadata["consumerID"]; ok && val != "" {
		meta.ConsumerGroup = val
		k.logger.Debugf("Using %s as ConsumerGroup", meta.ConsumerGroup)
	}

	if val, ok := metadata["consumerGroup"]; ok && val != "" {
		meta.ConsumerGroup = val
		k.logger.Debugf("Using %s as ConsumerGroup", meta.ConsumerGroup)
	}

	if val, ok := metadata["clientID"]; ok && val != "" {
		meta.ClientID = val
		k.logger.Debugf("Using %s as ClientID", meta.ClientID)
	}

	if val, ok := metadata["saslMechanism"]; ok && val != "" {
		meta.SaslMechanism = val
		k.logger.Debugf("Using %s as saslMechanism", meta.SaslMechanism)
	}

	initialOffset, err := parseInitialOffset(metadata["initialOffset"])
	if err != nil {
		return nil, err
	}
	meta.InitialOffset = initialOffset

	if val, ok := metadata["brokers"]; ok && val != "" {
		meta.Brokers = strings.Split(val, ",")
	} else {
		return nil, errors.New("kafka error: missing 'brokers' attribute")
	}

	k.logger.Debugf("Found brokers: %v", meta.Brokers)

	val, ok := metadata["authType"]
	if !ok {
		return nil, errors.New("kafka error: missing 'authType' attribute")
	}
	if val == "" {
		return nil, errors.New("kafka error: 'authType' attribute was empty")
	}

	switch strings.ToLower(val) {
	case passwordAuthType:
		meta.AuthType = val
		if val, ok = metadata["saslUsername"]; ok && val != "" {
			meta.SaslUsername = val
		} else {
			return nil, errors.New("kafka error: missing SASL Username for authType 'password'")
		}

		if val, ok = metadata["saslPassword"]; ok && val != "" {
			meta.SaslPassword = val
		} else {
			return nil, errors.New("kafka error: missing SASL Password for authType 'password'")
		}
		k.logger.Debug("Configuring SASL password authentication.")
	case oidcAuthType:
		meta.AuthType = val
		if val, ok = metadata["oidcTokenEndpoint"]; ok && val != "" {
			meta.OidcTokenEndpoint = val
		} else {
			return nil, errors.New("kafka error: missing OIDC Token Endpoint for authType 'oidc'")
		}
		if val, ok = metadata["oidcClientID"]; ok && val != "" {
			meta.OidcClientID = val
		} else {
			return nil, errors.New("kafka error: missing OIDC Client ID for authType 'oidc'")
		}
		if val, ok = metadata["oidcClientSecret"]; ok && val != "" {
			meta.OidcClientSecret = val
		} else {
			return nil, errors.New("kafka error: missing OIDC Client Secret for authType 'oidc'")
		}
		if val, ok = metadata["oidcScopes"]; ok && val != "" {
			meta.OidcScopes = strings.Split(val, ",")
		} else {
			k.logger.Warn("Warning: no OIDC scopes specified, using default 'openid' scope only. This is a security risk for token reuse.")
			meta.OidcScopes = []string{"openid"}
		}
		k.logger.Debug("Configuring SASL token authentication via OIDC.")
	case mtlsAuthType:
		meta.AuthType = val
		if val, ok = metadata[clientCert]; ok && val != "" {
			if !isValidPEM(val) {
				return nil, errors.New("kafka error: invalid client certificate")
			}
			meta.TLSClientCert = val
		}
		if val, ok = metadata[clientKey]; ok && val != "" {
			if !isValidPEM(val) {
				return nil, errors.New("kafka error: invalid client key")
			}
			meta.TLSClientKey = val
		}
		// clientKey and clientCert need to be all specified or all not specified.
		if (meta.TLSClientKey == "") != (meta.TLSClientCert == "") {
			return nil, errors.New("kafka error: clientKey or clientCert is missing")
		}
		k.logger.Debug("Configuring mTLS authentication.")
	case noAuthType:
		meta.AuthType = val
		k.logger.Debug("No authentication configured.")
	default:
		return nil, errors.New("kafka error: invalid value for 'authType' attribute")
	}

	if val, ok := metadata["maxMessageBytes"]; ok && val != "" {
		maxBytes, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("kafka error: cannot parse maxMessageBytes: %w", err)
		}

		meta.MaxMessageBytes = maxBytes
	}

	if val, ok := metadata[caCert]; ok && val != "" {
		if !isValidPEM(val) {
			return nil, errors.New("kafka error: invalid ca certificate")
		}
		meta.TLSCaCert = val
	}

	if val, ok := metadata["disableTls"]; ok && val != "" {
		boolVal, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("kafka: invalid value for 'tlsDisable' attribute: %w", err)
		}
		meta.TLSDisable = boolVal
		if meta.TLSDisable {
			k.logger.Info("kafka: TLS connectivity to broker disabled")
		}
	}

	if val, ok := metadata[skipVerify]; ok && val != "" {
		boolVal, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("kafka error: invalid value for '%s' attribute: %w", skipVerify, err)
		}
		meta.TLSSkipVerify = boolVal
		if boolVal {
			k.logger.Infof("kafka: you are using 'skipVerify' to skip server config verify which is unsafe!")
		}
	}

	if val, ok := metadata[consumeRetryEnabled]; ok && val != "" {
		boolVal, err := strconv.ParseBool(val)
		if err != nil {
			return nil, fmt.Errorf("kafka error: invalid value for '%s' attribute: %w", consumeRetryEnabled, err)
		}
		meta.ConsumeRetryEnabled = boolVal
	}

	if val, ok := metadata[consumeRetryInterval]; ok && val != "" {
		durationVal, err := time.ParseDuration(val)
		if err != nil {
			intVal, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("kafka error: invalid value for '%s' attribute: %w", consumeRetryInterval, err)
			}
			durationVal = time.Duration(intVal) * time.Millisecond
		}
		meta.ConsumeRetryInterval = durationVal
	}

	if val, ok := metadata["version"]; ok && val != "" {
		version, err := sarama.ParseKafkaVersion(val)
		if err != nil {
			return nil, errors.New("kafka error: invalid kafka version")
		}
		meta.Version = version
	} else {
		meta.Version = sarama.V2_0_0_0 //nolint:nosnakecase
	}

	return &meta, nil
}
