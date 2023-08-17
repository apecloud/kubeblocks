// Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

#PulsarProxyParameter: {
	// The ZooKeeper quorum connection string (as a comma-separated list)
	// @deprecated
	zookeeperServers:	string

	// The metadata store URL. \n Examples: \n  * zk:my-zk-1:2181,my-zk-2:2181,my-zk-3:2181\n  * my-zk-1:2181,my-zk-2:2181,my-zk-3:2181 (will default to ZooKeeper when the schema is not specified)\n  * zk:my-zk-1:2181,my-zk-2:2181,my-zk-3:2181/my-chroot-path (to add a ZK chroot path)\n
	metadataStoreUrl:	string

	// Configuration store connection string (as a comma-separated list). Deprecated in favor of `configurationMetadataStoreUrl`
	// @deprecated
	configurationStoreServers:	string

	// Global ZooKeeper quorum connection string (as a comma-separated list)
	// @deprecated
	globalZookeeperServers:	string

	// The metadata store URL for the configuration data. If empty, we fall back to use metadataStoreUrl
	configurationMetadataStoreUrl:	string

	// Metadata store session timeout in milliseconds.
	metadataStoreSessionTimeoutMillis:	int

	// Metadata store cache expiry time in seconds.
	metadataStoreCacheExpirySeconds:	int

	// Is metadata store read-only operations.
	metadataStoreAllowReadOnlyOperations:	bool

	// Max size of messages.
	maxMessageSize:	int

	// ZooKeeper session timeout in milliseconds. @deprecated - Use metadataStoreSessionTimeoutMillis instead.
	// @deprecated
	zookeeperSessionTimeoutMs:	int

	// ZooKeeper cache expiry time in seconds. @deprecated - Use metadataStoreCacheExpirySeconds instead.
	// @deprecated
	zooKeeperCacheExpirySeconds:	int

	// Is zooKeeper allow read-only operations.
	// @deprecated
	zooKeeperAllowReadOnlyOperations:	bool

	// The service url points to the broker cluster. URL must have the pulsar:// prefix.
	brokerServiceURL:	string

	// The tls service url points to the broker cluster. URL must have the pulsar+ssl:// prefix.
	brokerServiceURLTLS:	string

	// The web service url points to the broker cluster
	brokerWebServiceURL:	string

	// The tls web service url points to the broker cluster
	brokerWebServiceURLTLS:	string

	// The web service url points to the function worker cluster. Only configure it when you setup function workers in a separate cluster
	functionWorkerWebServiceURL:	string

	// The tls web service url points to the function worker cluster. Only configure it when you setup function workers in a separate cluster
	functionWorkerWebServiceURLTLS:	string

	// When enabled, checks that the target broker is active before connecting. zookeeperServers and configurationStoreServers must be configured in proxy configuration for retrieving the active brokers.
	checkActiveBrokers:	bool

	// Broker proxy connect timeout.\nThe timeout value for Broker proxy connect timeout is in millisecond. Set to 0 to disable.
	brokerProxyConnectTimeoutMs:	int

	// Broker proxy read timeout.\nThe timeout value for Broker proxy read timeout is in millisecond. Set to 0 to disable.
	brokerProxyReadTimeoutMs:	int

	// Allowed broker target host names. Supports multiple comma separated entries and a wildcard.
	brokerProxyAllowedHostNames:	string

	// Allowed broker target ip addresses or ip networks / netmasks. Supports multiple comma separated entries.
	brokerProxyAllowedIPAddresses:	string

	// Allowed broker target ports
	brokerProxyAllowedTargetPorts:	string

	// Hostname or IP address the service binds on
	bindAddress:	string

	// Hostname or IP address the service advertises to the outside world. If not set, the value of `InetAddress.getLocalHost().getCanonicalHostName()` is used.
	advertisedAddress:	string

	// Enable or disable the proxy protocol.
	haProxyProtocolEnabled:	bool

	// Enables zero-copy transport of data across network interfaces using the spice. Zero copy mode cannot be used when TLS is enabled or when proxyLogLevel is > 0.
	proxyZeroCopyModeEnabled:	bool

	// The port for serving binary protobuf request
	servicePort:	int

	// The port for serving tls secured binary protobuf request
	servicePortTls:	int

	// The port for serving http requests
	webServicePort:	int

	// The port for serving https requests
	webServicePortTls:	int

	// Specify the TLS provider for the web service, available values can be SunJSSE, Conscrypt and etc.
	webServiceTlsProvider:	string

	// Specify the tls protocols the proxy's web service will use to negotiate during TLS Handshake.\n\nExample:- [TLSv1.3, TLSv1.2]
	webServiceTlsProtocols:	string

	// Specify the tls cipher the proxy's web service will use to negotiate during TLS Handshake.\n\nExample:- [TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256]
	webServiceTlsCiphers:	string

	// The directory where nar Extraction happens
	narExtractionDirectory:	string

	// Proxy log level, default is 0. 0: Do not log any tcp channel info 1: Parse and log any tcp channel info and command info without message body 2: Parse and log channel info, command info and message body
	proxyLogLevel:	int

	// Path for the file used to determine the rotation status for the proxy instance when responding to service discovery health checks
	statusFilePath:	string

	// A list of role names (a comma-separated list of strings) that are treated as `super-user`, meaning they will be able to do all admin operations and publish & consume from all topics
	superUserRoles:	string

	// Whether authentication is enabled for the Pulsar proxy
	authenticationEnabled:	bool

	// Authentication provider name list (a comma-separated list of class names
	authenticationProviders:	string

	// Whether authorization is enforced by the Pulsar proxy
	authorizationEnabled:	bool

	// Authorization provider as a fully qualified class name
	authorizationProvider:	string

	// Whether client authorization credentials are forwarded to the broker for re-authorization.Authentication must be enabled via configuring `authenticationEnabled` to be true for thisto take effect
	forwardAuthorizationCredentials:	bool

	// Interval of time for checking for expired authentication credentials. Disable by setting to 0.
	authenticationRefreshCheckSeconds:	int

	// Whether the '/metrics' endpoint requires authentication. Defaults to true.'authenticationEnabled' must also be set for this to take effect.
	authenticateMetricsEndpoint:	bool

	// This is a regexp, which limits the range of possible ids which can connect to the Broker using SASL.\n Default value is: \".*pulsar.*\", so only clients whose id contains 'pulsar' are allowed to connect.
	saslJaasClientAllowedIds:	string

	// Service Principal, for login context name. Default value is \"PulsarProxy\".
	saslJaasServerSectionName:	string

	// Path to file containing the secret to be used to SaslRoleTokenSigner\nThe secret can be specified like:\nsaslJaasServerRoleTokenSignerSecretPath=file:///my/saslRoleTokenSignerSecret.key.
	saslJaasServerRoleTokenSignerSecretPath:	string

	// kerberos kinit command.
	kinitCommand:	string

	// Max concurrent inbound connections. The proxy will reject requests beyond that
	maxConcurrentInboundConnections:	int

	// The maximum number of connections per IP. If it exceeds, new connections are rejected.
	maxConcurrentInboundConnectionsPerIp:	int

	// Max concurrent lookup requests. The proxy will reject requests beyond that
	maxConcurrentLookupRequests:	int

	// The authentication plugin used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientAuthenticationPlugin:	string

	// The authentication parameters used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientAuthenticationParameters:	string

	// The path to trusted certificates used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientTrustCertsFilePath:	string

	// The path to TLS private key used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientKeyFilePath:	string

	// The path to the TLS certificate used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientCertificateFilePath:	string

	// Whether TLS is enabled when communicating with Pulsar brokers
	tlsEnabledWithBroker:	bool

	// When this parameter is not empty, unauthenticated users perform as anonymousUserRole
	anonymousUserRole:	string

	// Tls cert refresh duration in seconds (set 0 to check on every new connection)
	tlsCertRefreshCheckDurationSec:	int

	// Path for the TLS certificate file
	tlsCertificateFilePath:	string

	// Path for the TLS private key file
	tlsKeyFilePath:	string

	// Path for the trusted TLS certificate file.\n\nThis cert is used to verify that any certs presented by connecting clients are signed by a certificate authority. If this verification fails, then the certs are untrusted and the connections are dropped
	tlsTrustCertsFilePath:	string

	// Accept untrusted TLS certificate from client.\n\nIf true, a client with a cert which cannot be verified with the `tlsTrustCertsFilePath` cert will be allowed to connect to the server, though the cert will not be used for client authentication
	tlsAllowInsecureConnection:	bool

	// Whether the hostname is validated when the proxy creates a TLS connection with brokers
	tlsHostnameVerificationEnabled:	bool

	// Specify the tls protocols the broker will use to negotiate during TLS handshake (a comma-separated list of protocol names).\n\nExamples:- [TLSv1.3, TLSv1.2]
	tlsProtocols:	string

	// Specify the tls cipher the proxy will use to negotiate during TLS Handshake (a comma-separated list of ciphers).\n\nExamples:- [TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256]
	tlsCiphers:	string

	// Whether client certificates are required for TLS.\n\n Connections are rejected if the client certificate isn't trusted
	tlsRequireTrustedClientCertOnConnect:	bool

	// Enable TLS with KeyStore type configuration for proxy
	tlsEnabledWithKeyStore:	bool

	// Specify the TLS provider for the broker service: \nWhen using TLS authentication with CACert, the valid value is either OPENSSL or JDK.\nWhen using TLS authentication with KeyStore, available values can be SunJSSE, Conscrypt and etc.
	tlsProvider:	string

	// TLS KeyStore type configuration for proxy: JKS, PKCS12
	tlsKeyStoreType:	string

	// TLS KeyStore path for proxy
	tlsKeyStore:	string

	// TLS KeyStore password for proxy
	tlsKeyStorePassword:	string

	// TLS TrustStore type configuration for proxy: JKS, PKCS12
	tlsTrustStoreType:	string

	// TLS TrustStore path for proxy
	tlsTrustStore:	string

	// TLS TrustStore password for proxy, null means empty password.
	tlsTrustStorePassword:	string

	// Whether the Pulsar proxy use KeyStore type to authenticate with Pulsar brokers
	brokerClientTlsEnabledWithKeyStore:	bool

	// The TLS Provider used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientSslProvider:	string

	// TLS KeyStore type configuration for proxy: JKS, PKCS12  used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientTlsKeyStoreType:	string

	// TLS KeyStore path for internal client,  used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientTlsKeyStore:	string

	// TLS KeyStore password for proxy,  used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientTlsKeyStorePassword:	string

	// TLS TrustStore type configuration for proxy: JKS, PKCS12  used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientTlsTrustStoreType:	string

	// TLS TrustStore path for proxy,  used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientTlsTrustStore:	string

	// TLS TrustStore password for proxy,  used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientTlsTrustStorePassword:	string

	// Specify the tls cipher the proxy will use to negotiate during TLS Handshake (a comma-separated list of ciphers).\n\nExamples:- [TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256].\n used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientTlsCiphers:	string

	// Specify the tls protocols the broker will use to negotiate during TLS handshake (a comma-separated list of protocol names).\n\nExamples:- [TLSv1.3, TLSv1.2] \n used by the Pulsar proxy to authenticate with Pulsar brokers
	brokerClientTlsProtocols:	string

	// Http directs to redirect to non-pulsar services
	httpReverseProxyConfigs:	string

	// Http output buffer size.\n\nThe amount of data that will be buffered for http requests before it is flushed to the channel. A larger buffer size may result in higher http throughput though it may take longer for the client to see data. If using HTTP streaming via the reverse proxy, this should be set to the minimum value, 1, so that clients see the data as soon as possible.
	httpOutputBufferSize:	int

	// The maximum size in bytes of the request header.                Larger headers will allow for more and/or larger cookies plus larger form content encoded in a URL.                However, larger headers consume more memory and can make a server more vulnerable to denial of service                attacks.
	httpMaxRequestHeaderSize:	int

	// Http input buffer max size.\n\nThe maximum amount of data that will be buffered for incoming http requests so that the request body can be replayed when the backend broker issues a redirect response.
	httpInputMaxReplayBufferSize:	int

	// Http proxy timeout.\n\nThe timeout value for HTTP proxy is in millisecond.
	httpProxyTimeout:	int

	// Number of threads to use for HTTP requests processing
	httpNumThreads:	int

	// Max concurrent web requests
	maxConcurrentHttpRequests:	int

	// Capacity for thread pool queue in the HTTP server Default is set to 8192.
	httpServerThreadPoolQueueSize:	int

	// Capacity for accept queue in the HTTP server Default is set to 8192.
	httpServerAcceptQueueSize:	int

	// Maximum number of inbound http connections. (0 to disable limiting)
	maxHttpServerConnections:	int

	// Number of threads used for Netty IO. Default is set to `2 * Runtime.getRuntime().availableProcessors()`
	numIOThreads:	int

	// Number of threads used for Netty Acceptor. Default is set to `1`
	numAcceptorThreads:	int

	// The directory to locate proxy additional servlet
	proxyAdditionalServletDirectory:	string

	// The directory to locate proxy additional servlet
	additionalServletDirectory:	string

	// List of proxy additional servlet to load, which is a list of proxy additional servlet names
	proxyAdditionalServlets:	string

	// List of proxy additional servlet to load, which is a list of proxy additional servlet names
	additionalServlets:	string

	// Enable the enforcement of limits on the incoming HTTP requests
	httpRequestsLimitEnabled:	bool

	// Max HTTP requests per seconds allowed. The excess of requests will be rejected with HTTP code 429 (Too many requests)
	httpRequestsMaxPerSecond:	float

	// The directory to locate proxy extensions
	proxyExtensionsDirectory:	string

	// List of messaging protocols to load, which is a list of extension names
	proxyExtensions:	string

	// Use a separate ThreadPool for each Proxy Extension
	useSeparateThreadPoolForProxyExtensions:	bool

	// Enable or disable the WebSocket servlet
	webSocketServiceEnabled:	bool

	// Interval of time to sending the ping to keep alive in WebSocket proxy. This value greater than 0 means enabled
	webSocketPingDurationSeconds:	int

	// Name of the cluster to which this broker belongs to
	clusterName:	string

	...
}

configuration: #PulsarProxyParameter & {
}
