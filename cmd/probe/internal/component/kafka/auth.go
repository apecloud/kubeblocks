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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"

	"github.com/Shopify/sarama"
)

func updatePasswordAuthInfo(config *sarama.Config, metadata *kafkaMetadata, saslUsername, saslPassword string) {
	config.Net.SASL.Enable = true
	config.Net.SASL.User = saslUsername
	config.Net.SASL.Password = saslPassword
	if metadata.SaslMechanism == "SHA-256" {
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
		config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
	} else if metadata.SaslMechanism == "SHA-512" {
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
		config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	} else {
		config.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	}
}

func updateMTLSAuthInfo(config *sarama.Config, metadata *kafkaMetadata) error {
	if metadata.TLSDisable {
		return fmt.Errorf("kafka: cannot configure mTLS authentication when TLSDisable is 'true'")
	}
	cert, err := tls.X509KeyPair([]byte(metadata.TLSClientCert), []byte(metadata.TLSClientKey))
	if err != nil {
		return fmt.Errorf("unable to load client certificate and key pair. Err: %w", err)
	}
	config.Net.TLS.Config.Certificates = []tls.Certificate{cert}
	return nil
}

func updateTLSConfig(config *sarama.Config, metadata *kafkaMetadata) error {
	if metadata.TLSDisable || metadata.AuthType == noAuthType {
		config.Net.TLS.Enable = false
		return nil
	}
	config.Net.TLS.Enable = true

	if !metadata.TLSSkipVerify && metadata.TLSCaCert == "" {
		return nil
	}
	//nolint:gosec
	config.Net.TLS.Config = &tls.Config{InsecureSkipVerify: metadata.TLSSkipVerify, MinVersion: tls.VersionTLS12}
	if metadata.TLSCaCert != "" {
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM([]byte(metadata.TLSCaCert)); !ok {
			return errors.New("kafka error: unable to load ca certificate")
		}
		config.Net.TLS.Config.RootCAs = caCertPool
	}

	return nil
}

func updateOidcAuthInfo(config *sarama.Config, metadata *kafkaMetadata) error {
	tokenProvider := newOAuthTokenSource(metadata.OidcTokenEndpoint, metadata.OidcClientID, metadata.OidcClientSecret, metadata.OidcScopes)

	if metadata.TLSCaCert != "" {
		err := tokenProvider.addCa(metadata.TLSCaCert)
		if err != nil {
			return fmt.Errorf("kafka: error setting oauth client trusted CA: %w", err)
		}
	}

	tokenProvider.skipCaVerify = metadata.TLSSkipVerify

	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = sarama.SASLTypeOAuth
	config.Net.SASL.TokenProvider = &tokenProvider

	return nil
}
