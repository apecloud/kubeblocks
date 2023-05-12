#!/bin/bash

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

if [[ -n "$KAFKA_KRAFT_CLUSTER_ID" ]]; then
    kraft_id_len=${#KAFKA_KRAFT_CLUSTER_ID}
    if [[ kraft_id_len > 22 ]]; then
        export KAFKA_KRAFT_CLUSTER_ID=$(echo $KAFKA_KRAFT_CLUSTER_ID | cut -b 1-22)
        echo export KAFKA_KRAFT_CLUSTER_ID="${KAFKA_KRAFT_CLUSTER_ID}"
    fi
fi

{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}

if [[ "broker" = "$KAFKA_CFG_PROCESS_ROLES" ]]; then
    # override node.id setting
    # increments based on a specified base to avoid conflicts with controller settings
    INDEX=$(echo $KB_POD_NAME | grep -o "\-[0-9]\+\$")
    INDEX=${INDEX#-}
    BROKER_NODE_ID=$(( $INDEX + $BROKER_MIN_NODE_ID ))
    export KAFKA_CFG_NODE_ID="$BROKER_NODE_ID"
    echo "KAFKA_CFG_NODE_ID=$KAFKA_CFG_NODE_ID"
    # generate KAFKA_CFG_CONTROLLER_QUORUM_VOTERS for broker if not a combine-cluster
    {{- $voters := "" }}
    {{- range $i, $c := $.cluster.spec.componentSpecs }}
      {{- if eq "kafka-controller" $c.componentDefRef }}
        {{- $replicas := $c.replicas | int }}
        {{- range $n, $e := until $replicas }}
          {{- $podFQDN := printf "%s-%s-%d.%s-%s-headless.%s.svc.cluster.local" $clusterName $c.name $n $clusterName $c.name $namespace }} # Todo: cluster.local
          {{- $voter := printf "%d@%s:9093" ( $n | int ) $podFQDN }}
          {{- $voters = printf "%s,%s" $voters $voter }}
        {{- end }}
        {{- $voters = trimPrefix "," $voters }}
      {{- end }}
    {{- end }}
    export KAFKA_CFG_CONTROLLER_QUORUM_VOTERS={{ $voters }}
    echo "export KAFKA_CFG_CONTROLLER_QUORUM_VOTERS=$KAFKA_CFG_CONTROLLER_QUORUM_VOTERS,for kafka-broker."
else
    # generate node.id
    ID="${KB_POD_NAME#${KB_CLUSTER_COMP_NAME}-}"
    export KAFKA_CFG_NODE_ID="$((ID + 0))"
    echo "KAFKA_CFG_NODE_ID=$KAFKA_CFG_NODE_ID"
    # generate KAFKA_CFG_CONTROLLER_QUORUM_VOTERS if is a combine-cluster or controller
    {{- $replicas := $.component.replicas | int }}
    {{- $voters := "" }}
    {{- range $i, $e := until $replicas }}
      {{- $podFQDN := printf "%s-%s-%d.%s-%s-headless.%s.svc.cluster.local" $clusterName $.component.name $i $clusterName $.component.name $namespace }}
      {{- $voter := printf "%d@%s:9093" ( $i | int ) $podFQDN }}
      {{- $voters = printf "%s,%s" $voters $voter }}
    {{- end }}
    {{- $voters = trimPrefix "," $voters }}
    export KAFKA_CFG_CONTROLLER_QUORUM_VOTERS={{ $voters }}
    echo "export KAFKA_CFG_CONTROLLER_QUORUM_VOTERS=$KAFKA_CFG_CONTROLLER_QUORUM_VOTERS,for kafka-server."
fi

exec /entrypoint.sh /run.sh