#!/bin/sh

until mysql -h$GMS_SVC_NAME -P$GMS_SVC_PORT -u$metaDbUser -p$metaDbNonEncPasswd -e 'select 1'; do
    sleep 1;
    echo "wait gms ready"
done

function generate_dn_init_sql() {
    echo "$DN_HEADLESS_SVC_NAME" | tr ',' '\n' | while IFS= read -r item
    do
      DN_HOSTNAME=$item
      DN_NAME=$(echo "$DN_HOSTNAME" | cut -d'.' -f2 | sed s/-headless//)
      dn_init_sql="INSERT IGNORE INTO storage_info (id, gmt_created, gmt_modified, inst_id, storage_inst_id, storage_master_inst_id,ip, port, xport, user, passwd_enc, storage_type, inst_kind, status, region_id, azone_id, idc_id, max_conn, cpu_core, mem_size, is_vip, extras)
      VALUES (NULL, NOW(), NOW(), '$KB_CLUSTER_NAME', '$DN_NAME', '$DN_NAME', '$DN_HOSTNAME', '3306', '31600', '$metaDbUser', '$ENC_PASSWORD', '3', '0', '0', NULL, NULL, NULL, 10000, 4,  34359738368 , '0', '');"
      echo $dn_init_sql >> /scripts/gms-init-metadata.sql
    done
    echo "UPDATE config_listener SET op_version = op_version + 1 WHERE data_id = 'polardbx.storage.info.$KB_CLUSTER_NAME'" >> /scripts/gms-init-metadata.sql
}

ENC_PASSWORD=$(echo -n "$metaDbNonEncPasswd" | openssl enc -aes-128-ecb -K "$(printf "%s" "$dnPasswordKey" | od -An -tx1 | tr -d " \n")" -base64)
SHA1_ENC_PASSWORD=$(echo -n "$metaDbNonEncPasswd" | sha1sum | cut -d ' ' -f1)
echo "export metaDbPasswd=$ENC_PASSWORD" >> /shared/env.sh

SOURCE_CMD="mysql -h$GMS_SVC_NAME -P$GMS_SVC_PORT -u$metaDbUser -p$metaDbNonEncPasswd -e 'source /scripts/gms-init.sql'"
eval $SOURCE_CMD

GMS_HOST=$GMS_SVC_NAME"."$KB_NAMESPACE".svc.cluster.local"

eval "gms_metadata_sql=\"$(cat /scripts/gms-metadata.tpl)\""

echo $gms_metadata_sql > /scripts/gms-init-metadata.sql
generate_dn_init_sql

cat /scripts/gms-init-metadata.sql

eval "mysql -h$GMS_SVC_NAME -P$GMS_SVC_PORT -u$metaDbUser -p$metaDbNonEncPasswd -e 'source /scripts/gms-init-metadata.sql'"

