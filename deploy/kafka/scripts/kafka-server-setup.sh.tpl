#!/bin/bash

# TLS setting
{{- if $.component.tls }}
  # override TLS and auth settings
  export KAFKA_TLS_TYPE="PEM"
  echo "[tls]KAFKA_TLS_TYPE=$KAFKA_TLS_TYPE"
  export KAFKA_CFG_SSL_ENDPOINT_IDENTIFICATION_ALGORITHM=""
  echo "[tls]KAFKA_CFG_SSL_ENDPOINT_IDENTIFICATION_ALGORITHM=$KAFKA_CFG_SSL_ENDPOINT_IDENTIFICATION_ALGORITHM"
  export KAFKA_CERTIFICATE_PASSWORD=""
  echo "[tls]KAFKA_CERTIFICATE_PASSWORD=$KAFKA_CERTIFICATE_PASSWORD"
  export KAFKA_TLS_CLIENT_AUTH=none
  echo "[tls]KAFKA_TLS_CLIENT_AUTH=$KAFKA_TLS_CLIENT_AUTH"

  # override TLS protocol
  export KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,INTERNAL:PLAINTEXT,CLIENT:SSL
  echo "KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=$KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP"
  # Todo: enable encrypted transmission inside the service
  #export KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:SSL,INTERNAL:SSL,CLIENT:SSL
  #export KAFKA_CFG_SECURITY_INTER_BROKER_PROTOCOL=SSL
  #echo "KAFKA_CFG_SECURITY_INTER_BROKER_PROTOCOL=SSL"

  mkdir -p /opt/bitnami/kafka/config/certs
  PEM_CA="$KB_TLS_CERT_PATH/ca.crt"
  PEM_CERT="$KB_TLS_CERT_PATH/tls.crt"
  PEM_KEY="$KB_TLS_CERT_PATH/tls.key"
  if [[ -f "$PEM_CERT" ]] && [[ -f "$PEM_KEY" ]]; then
      CERT_DIR="/opt/bitnami/kafka/config/certs"
      PEM_CA_LOCATION="${CERT_DIR}/kafka.truststore.pem"
      PEM_CERT_LOCATION="${CERT_DIR}/kafka.keystore.pem"
          if [[ -f "$PEM_CA" ]]; then
              cp "$PEM_CA" "$PEM_CA_LOCATION"
              cp "$PEM_CERT" "$PEM_CERT_LOCATION"
          else
              echo "[tls]PEM_CA not provided, and auth.tls.pemChainIncluded was not true. One of these values must be set when using PEM type for TLS."
              exit 1
          fi

      # Ensure the key used PEM format with PKCS#8
      openssl pkcs8 -topk8 -nocrypt -in "$PEM_KEY" > "${CERT_DIR}/kafka.keystore.key"
      # combined the certificate and private-key for client use
      cat ${CERT_DIR}/kafka.keystore.key ${PEM_CERT_LOCATION} > ${CERT_DIR}/client.combined.key
  else
      echo "[tls]Couldn't find the expected PEM files! They are mandatory when encryption via TLS is enabled."
      exit 1
  fi
  export KAFKA_TLS_TRUSTSTORE_FILE="/opt/bitnami/kafka/config/certs/kafka.truststore.pem"
  echo "[tls]KAFKA_TLS_TRUSTSTORE_FILE=$KAFKA_TLS_TRUSTSTORE_FILE"
  echo "[tls]ssl.endpoint.identification.algorithm=" >> /opt/bitnami/kafka/config/kraft/server.properties
  echo "[tls]ssl.endpoint.identification.algorithm=" >> /opt/bitnami/kafka/config/server.properties
{{- end }}

# cfg setting with props
# convert server.properties to 'export KAFKA_CFG_{prop}' env variables
SERVER_PROP_PATH=${SERVER_PROP_PATH:-/bitnami/kafka/config/server.properties}
SERVER_PROP_FILE=${SERVER_PROP_FILE:-server.properties}

if [[ -f "$SERVER_PROP_FILE" ]]; then
    IFS='='
    while read -r line; do
        if [[ "$line" =~ ^#.* ]]; then
          continue
        fi
        echo "convert prop ${line}"
        read -ra kv <<< "$line"
        len=${#kv[@]}
        if [[ $len != 2 ]]; then
            echo "line '${line}' has no value; skipped"
            continue
        fi
        env_suffix=${kv[0]^^}
        env_suffix=${env_suffix//./_}
        env_suffix=`eval echo "${env_suffix}"`
        env_value=`eval echo "${kv[1]}"`
        export KAFKA_CFG_${env_suffix}="${env_value}"
        echo "[cfg]export KAFKA_CFG_${env_suffix}=${env_value}"
    done <$SERVER_PROP_FILE
    unset IFS
fi

# override SASL settings
if [[ "true" == "$KB_KAFKA_ENABLE_SASL" ]]; then
  # bitnami default jaas setting: /opt/bitnami/kafka/config/kafka_jaas.conf
  if [[ "${KB_KAFKA_SASL_CONFIG_PATH}" ]]; then
    cp ${KB_KAFKA_SASL_CONFIG_PATH} /opt/bitnami/kafka/config/kafka_jaas.conf
    echo "[sasl]do: cp ${KB_KAFKA_SASL_CONFIG_PATH} /opt/bitnami/kafka/config/kafka_jaas.conf "
  fi
  export KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,INTERNAL:SASL_PLAINTEXT,CLIENT:SASL_PLAINTEXT
  echo "[sasl]KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=$KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP"
  export KAFKA_CFG_SASL_ENABLED_MECHANISMS="PLAIN"
  echo "[sasl]export KAFKA_CFG_SASL_ENABLED_MECHANISMS=${KAFKA_CFG_SASL_ENABLED_MECHANISMS}"
  export KAFKA_CFG_SASL_MECHANISM_INTER_BROKER_PROTOCOL="PLAIN"
  echo "[sasl]export KAFKA_CFG_SASL_MECHANISM_INTER_BROKER_PROTOCOL=${KAFKA_CFG_SASL_MECHANISM_INTER_BROKER_PROTOCOL}"
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

# jvm setting
if [[ -n "$KB_KAFKA_BROKER_HEAP" ]]; then
  export KAFKA_HEAP_OPTS=${KB_KAFKA_BROKER_HEAP}
  echo "[jvm][KB_KAFKA_BROKER_HEAP]export KAFKA_HEAP_OPTS=${KB_KAFKA_BROKER_HEAP}"
fi

# cfg setting
if [[ "broker" = "$KAFKA_CFG_PROCESS_ROLES" ]]; then
    # override node.id setting
    # increments based on a specified base to avoid conflicts with controller settings
    INDEX=$(echo $KB_POD_NAME | grep -o "\-[0-9]\+\$")
    INDEX=${INDEX#-}
    BROKER_NODE_ID=$(( $INDEX + $BROKER_MIN_NODE_ID ))
    export KAFKA_CFG_NODE_ID="$BROKER_NODE_ID"
    export KAFKA_CFG_BROKER_ID="$BROKER_NODE_ID"
    echo "[cfg]KAFKA_CFG_NODE_ID=$KAFKA_CFG_NODE_ID"
    # generate KAFKA_CFG_CONTROLLER_QUORUM_VOTERS for broker if not a combine-cluster
    {{- $voters := "" }}
    {{- range $i, $c := $.cluster.spec.componentSpecs }}
      {{- if eq "controller" $c.componentDefRef }}
        {{- $replicas := $c.replicas | int }}
        {{- range $n, $e := until $replicas }}
          {{- $podFQDN := printf "%s-%s-%d.%s-%s-headless.%s.svc.cluster.local" $clusterName $c.name $n $clusterName $c.name $namespace }} #
          {{- $voter := printf "%d@%s:9093" ( $n | int ) $podFQDN }}
          {{- $voters = printf "%s,%s" $voters $voter }}
        {{- end }}
        {{- $voters = trimPrefix "," $voters }}
      {{- end }}
    {{- end }}
    export KAFKA_CFG_CONTROLLER_QUORUM_VOTERS={{ $voters }}
    echo "[cfg]export KAFKA_CFG_CONTROLLER_QUORUM_VOTERS=$KAFKA_CFG_CONTROLLER_QUORUM_VOTERS,for kafka-broker."

    # deleting this information can reacquire the controller members when the broker restarts,
    # and avoid the mismatch between the controller in the quorum-state and the actual controller in the case of a controller scale,
    # which will cause the broker to fail to start
    if [ -f "$KAFKA_CFG_METADATA_LOG_DIR/__cluster_metadata-0/quorum-state" ]; then
      echo "[action]Removing quorum-state file when restart."
      rm -f "$KAFKA_CFG_METADATA_LOG_DIR/__cluster_metadata-0/quorum-state"
    fi
else
    if [[ "controller" = "$KAFKA_CFG_PROCESS_ROLES" ]] && [[ -n "$KB_KAFKA_CONTROLLER_HEAP" ]]; then
      export KAFKA_HEAP_OPTS=${KB_KAFKA_CONTROLLER_HEAP}
      echo "[jvm][KB_KAFKA_CONTROLLER_HEAP]export KAFKA_HEAP_OPTS=${KB_KAFKA_CONTROLLER_HEAP}"
    fi
    # generate node.id
    ID="${KB_POD_NAME#${KB_CLUSTER_COMP_NAME}-}"
    export KAFKA_CFG_NODE_ID="$((ID + 0))"
    export KAFKA_CFG_BROKER_ID="$((ID + 0))"
    echo "[cfg]KAFKA_CFG_NODE_ID=$KAFKA_CFG_NODE_ID"
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
    echo "[cfg]export KAFKA_CFG_CONTROLLER_QUORUM_VOTERS=$KAFKA_CFG_CONTROLLER_QUORUM_VOTERS,for kafka-server."
fi

exec /entrypoint.sh /run.sh