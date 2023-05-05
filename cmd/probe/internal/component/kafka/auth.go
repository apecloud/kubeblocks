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
	switch metadata.SaslMechanism {
	case "SHA-256":
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
		config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
	case "SHA-512":
		config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
		config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	default:
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
