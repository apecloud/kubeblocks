USE polardbx_meta_db;

INSERT IGNORE INTO schema_change(table_name, version) VALUES('user_priv', 1);
INSERT IGNORE INTO quarantine_config VALUES (NULL, NOW(), NOW(), '$KB_CLUSTER_NAME', 'default', NULL, NULL, '0.0.0.0/0');

INSERT IGNORE INTO config_listener (id, gmt_created, gmt_modified, data_id, status, op_version, extras) VALUES (NULL, NOW(), NOW(), 'polardbx.server.info.$KB_CLUSTER_NAME', 0, 0, NULL);
INSERT IGNORE INTO config_listener (id, gmt_created, gmt_modified, data_id, status, op_version, extras) VALUES (NULL, NOW(), NOW(), 'polardbx.storage.info.$KB_CLUSTER_NAME', 0, 0, NULL);
INSERT IGNORE INTO config_listener (id, gmt_created, gmt_modified, data_id, status, op_version, extras) VALUES (NULL, NOW(), NOW(), 'polardbx.inst.config.$KB_CLUSTER_NAME', 0, 0, NULL);
INSERT IGNORE INTO config_listener (id, gmt_created, gmt_modified, data_id, status, op_version, extras) VALUES (NULL, NOW(), NOW(), 'polardbx.quarantine.config.$KB_CLUSTER_NAME', 0, 0, NULL);
INSERT IGNORE INTO config_listener (id, gmt_created, gmt_modified, data_id, status, op_version, extras) VALUES (NULL, NOW(), NOW(), 'polardbx.privilege.info', 0, 0, NULL);

INSERT IGNORE INTO inst_config (inst_id, param_key,param_val) values ('$KB_CLUSTER_NAME','CONN_POOL_XPROTO_META_DB_PORT','0');
INSERT IGNORE INTO inst_config (inst_id, param_key,param_val) values ('$KB_CLUSTER_NAME','CDC_STARTUP_MODE','1');
INSERT IGNORE INTO inst_config (inst_id, param_key,param_val) values ('$KB_CLUSTER_NAME','CONN_POOL_MAX_POOL_SIZE','500');
INSERT IGNORE INTO inst_config (inst_id, param_key,param_val) values ('$KB_CLUSTER_NAME','MAX_PREPARED_STMT_COUNT','500000');

INSERT IGNORE INTO storage_info (id, gmt_created, gmt_modified, inst_id, storage_inst_id, storage_master_inst_id,ip, port, xport, user, passwd_enc, storage_type, inst_kind, status, region_id, azone_id, idc_id, max_conn, cpu_core, mem_size, is_vip, extras)
    VALUES (NULL, NOW(), NOW(), '$KB_CLUSTER_NAME', '$GMS_SVC_NAME', '$GMS_SVC_NAME', '$GMS_HOST', '3306', '31600', '$metaDbUser', '$ENC_PASSWORD', '3', '2', '0', NULL, NULL, NULL, 10000, 4,  34359738368 , '0', '');

INSERT IGNORE INTO user_priv (id, gmt_created, gmt_modified, user_name, host, password, select_priv, insert_priv, update_priv, delete_priv, create_priv, drop_priv, grant_priv, index_priv, alter_priv, show_view_priv, create_view_priv, create_user_priv, meta_db_priv)
    VALUES (NULL, now(), now(), '$metaDbUser', '%', '$SHA1_ENC_PASSWORD', '1', '1', '1', '1', '1', '1', '1', '1', '1', '1', '1', '1', '1');

UPDATE config_listener SET op_version = op_version + 1 WHERE data_id = 'polardbx.privilege.info';