#!/bin/bash
ID="${KB_POD_NAME#${KB_CLUSTER_COMP_NAME}-}"
export KAFKA_CFG_BROKER_ID="$((ID + 0))"
echo "KAFKA_CFG_BROKER_ID=$KAFKA_CFG_BROKER_ID"

{{- if $.component.tls }}
# override TLS and auth settings
export KAFKA_TLS_TYPE="PEM"
echo "KAFKA_TLS_TYPE=$KAFKA_TLS_TYPE"
export KAFKA_CFG_SSL_ENDPOINT_IDENTIFICATION_ALGORITHM="https"
echo "KAFKA_CFG_SSL_ENDPOINT_IDENTIFICATION_ALGORITHM=$KAFKA_CFG_SSL_ENDPOINT_IDENTIFICATION_ALGORITHM"
export KAFKA_CERTIFICATE_PASSWORD=""
echo "KAFKA_CERTIFICATE_PASSWORD=$KAFKA_CERTIFICATE_PASSWORD"
export KAFKA_TLS_CLIENT_AUTH=required
echo "KAFKA_TLS_CLIENT_AUTH=$KAFKA_TLS_CLIENT_AUTH"
export KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:SSL,INTERNAL:PLAINTEXT,CLIENT:SSL
echo "KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=$KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP"

mkdir -p /opt/bitnami/kafka/config/certs
PEM_CA="/certs-${ID}/ca.crt"
PEM_CERT="/certs-${ID}/tls.crt"
PEM_KEY="/certs-${ID}/tls.key"
if [[ -f "$PEM_CERT" ]] && [[ -f "$PEM_KEY" ]]; then
    CERT_DIR="/opt/bitnami/kafka/config/certs"
    PEM_CA_LOCATION="${CERT_DIR}/kafka.truststore.pem"
    PEM_CERT_LOCATION="${CERT_DIR}/kafka.keystore.pem"
        if [[ -f "$PEM_CA" ]]; then
            cp "$PEM_CA" "$PEM_CA_LOCATION"
            cp "$PEM_CERT" "$PEM_CERT_LOCATION"
        else
            echo "PEM_CA not provided, and auth.tls.pemChainIncluded was not true. One of these values must be set when using PEM type for TLS."
            exit 1
        fi

    # Ensure the key used PEM format with PKCS#8
    openssl pkcs8 -topk8 -nocrypt -in "$PEM_KEY" > "/opt/bitnami/kafka/config/certs/kafka.keystore.key"
else
    echo "Couldn't find the expected PEM files! They are mandatory when encryption via TLS is enabled."
    exit 1
fi
export KAFKA_TLS_TRUSTSTORE_FILE="/opt/bitnami/kafka/config/certs/kafka.truststore.pem"
echo "KAFKA_TLS_TRUSTSTORE_FILE=$KAFKA_TLS_TRUSTSTORE_FILE"
{{- end }}

# convert server.properties to 'export KAFKA_CFG_{prop}' env variables
SERVER_PROP_PATH=${SERVER_PROP_PATH:-/bitnami/kafka/config/server.properties}
SERVER_PROP_FILE=${SERVER_PROP_FILE:-server.properties}
if [[ -f "$SERVER_PROP_FILE" ]]; then
    IFS='='
    while read -r line; do
        echo "convert prop ${line}"
        read -ra kv <<< "$line"
        len=${#kv[@]}
        if [[ $len != 2 ]]; then
            echo "line '${line}' has no value; skipped"
            continue
        fi
        env_suffix=${kv[0]^^}
        env_suffix=${env_suffix//./_}
        export KAFKA_CFG_${env_suffix}="${kv[1]}"
        echo "export KAFKA_CFG_${env_suffix}=${kv[1]}"
    done <$SERVER_PROP_FILE
    unset IFS
fi

{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $component := fromJson "{}" }}
{{- if eq "broker" ( getEnvByName ( index $.component.podSpec.containers 0 ) "KAFKA_CFG_PROCESS_ROLES" ) }}
  {{- $component = $.component }}
{{- /* build KAFKA_CFG_CONTROLLER_QUORUM_VOTERS value string */}}
{{- $replicas := $.component.replicas | int }}
{{- $voters := "" }}
{{- range $i, $e := until $replicas }}
  {{- $podFQDN := printf "%s-%s-%d.%s-%s-headless.%s.svc" $clusterName $.component.name $i $clusterName $.component.name $namespace }}
  {{- $voter := printf "%d@%s:9093" ( $i | int | add1 ) $podFQDN }}
  {{- $voters = printf "%s,%s" $voters $voter }}
{{- end }}
{{- $voters = trimPrefix "," $voters }}
export KAFKA_CFG_CONTROLLER_QUORUM_VOTERS={{ $voters }}
echo "export KAFKA_CFG_CONTROLLER_QUORUM_VOTERS=$KAFKA_CFG_CONTROLLER_QUORUM_VOTERS"
{{- end }}

if [[ -n "$KAFKA_KRAFT_CLUSTER_ID" ]]; then
    kraft_id_len=${#KAFKA_KRAFT_CLUSTER_ID}
    if [[ kraft_id_len > 22 ]]; then
        export KAFKA_KRAFT_CLUSTER_ID=$(echo $KAFKA_KRAFT_CLUSTER_ID | cut -b 1-22)
        echo export KAFKA_KRAFT_CLUSTER_ID="${KAFKA_KRAFT_CLUSTER_ID}"
    fi
fi

exec /entrypoint.sh /run.sh