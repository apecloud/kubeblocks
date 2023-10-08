CREATE DATABASE IF NOT EXISTS polardbx_meta_db;
USE polardbx_meta_db;

CREATE TABLE IF NOT EXISTS server_info (
  id BIGINT(11) NOT NULL auto_increment,
  gmt_created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  gmt_modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP on UPDATE CURRENT_TIMESTAMP,
  inst_id VARCHAR(128) NOT NULL,
  inst_type INT(11) NOT NULL,
  ip VARCHAR(128) NOT NULL,
  port INT(11) NOT NULL,
  htap_port INT(11) NOT NULL,
  mgr_port INT(11) NOT NULL,
  mpp_port INT(11) NOT NULL,
  status INT(11) NOT NULL,
  region_id VARCHAR(128) DEFAULT NULL,
  azone_id VARCHAR(128) DEFAULT NULL,
  idc_id VARCHAR(128) DEFAULT NULL,
  cpu_core INT(11) DEFAULT NULL,
  mem_size INT(11) DEFAULT NULL,
  extras text DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_inst_id_addr (inst_id, ip, port),
  INDEX idx_inst_id_status (inst_id, status)
) engine = innodb DEFAULT charset = utf8;

CREATE TABLE IF NOT EXISTS storage_info (
  id BIGINT(11) NOT NULL auto_increment,
  gmt_created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  gmt_modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP on UPDATE CURRENT_TIMESTAMP,
  inst_id VARCHAR(128) NOT NULL,
  storage_inst_id VARCHAR(128) NOT NULL,
  storage_master_inst_id VARCHAR(128) NOT NULL,
  ip VARCHAR(128) NOT NULL,
  port INT(11) NOT NULL comment 'port for mysql',
  xport INT(11) DEFAULT NULL comment 'port for x-protocol',
  user VARCHAR(128) NOT NULL,
  passwd_enc text NOT NULL,
  storage_type INT(11) NOT NULL comment '0:x-cluster, 1:mysql, 2:polardb',
  inst_kind INT(11) NOT NULL comment '0:master, 1:slave, 2:metadb',
  status INT(11) NOT NULL comment '0:storage ready, 1:storage not_ready',
  region_id VARCHAR(128) DEFAULT NULL,
  azone_id VARCHAR(128) DEFAULT NULL,
  idc_id VARCHAR(128) DEFAULT NULL,
  max_conn INT(11) NOT NULL,
  cpu_core INT(11) DEFAULT NULL,
  mem_size INT(11) DEFAULT NULL comment 'mem unit: MB',
  is_vip INT(11) DEFAULT NULL COMMENT '0:ip is NOT vip, 1:ip is vip',
  extras text DEFAULT NULL COMMENT 'reserve for extra info',
  PRIMARY KEY (id),
  INDEX idx_inst_id_status (inst_id, status),
  UNIQUE KEY uk_inst_id_addr (storage_inst_id, ip, port, inst_kind)
) engine = innodb DEFAULT charset = utf8;

CREATE TABLE if not exists user_priv (
  id bigint(11) NOT NULL AUTO_INCREMENT,
  gmt_created timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  gmt_modified timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  user_name char(32) COLLATE utf8_unicode_ci NOT NULL DEFAULT '',
  host char(60) COLLATE utf8_unicode_ci NOT NULL DEFAULT '',
  password char(100) COLLATE utf8_unicode_ci NOT NULL,
  select_priv tinyint(1) NOT NULL DEFAULT '0',
  insert_priv tinyint(1) NOT NULL DEFAULT '0',
  update_priv tinyint(1) NOT NULL DEFAULT '0',
  delete_priv tinyint(1) NOT NULL DEFAULT '0',
  create_priv tinyint(1) NOT NULL DEFAULT '0',
  drop_priv tinyint(1) NOT NULL DEFAULT '0',
  grant_priv tinyint(1) NOT NULL DEFAULT '0',
  index_priv tinyint(1) NOT NULL DEFAULT '0',
  alter_priv tinyint(1) NOT NULL DEFAULT '0',
  show_view_priv int(11) NOT NULL DEFAULT '0',
  create_view_priv int(11) NOT NULL DEFAULT '0',
  create_user_priv int(11) NOT NULL DEFAULT '0',
  meta_db_priv int(11) NOT NULL DEFAULT '0',
  PRIMARY KEY (id),
  UNIQUE KEY uk (user_name, host)
) ENGINE = InnoDB DEFAULT CHARSET = utf8 COLLATE = utf8_unicode_ci COMMENT = 'Users and global privileges';

CREATE TABLE IF NOT EXISTS quarantine_config (
  id BIGINT(11) NOT NULL auto_increment,
  gmt_created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  gmt_modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP on UPDATE CURRENT_TIMESTAMP,
  inst_id VARCHAR(100) CHARACTER SET utf8 COLLATE utf8_unicode_ci NOT NULL,
  group_name VARCHAR(200) CHARACTER SET utf8 COLLATE utf8_unicode_ci NOT NULL,
  net_work_type VARCHAR(100) CHARACTER SET utf8 COLLATE utf8_unicode_ci DEFAULT NULL,
  security_ip_type VARCHAR(100) CHARACTER SET utf8 COLLATE utf8_unicode_ci DEFAULT NULL,
  security_ips text CHARACTER SET utf8 COLLATE utf8_unicode_ci,
  PRIMARY KEY (id),
  UNIQUE KEY uk (inst_id, group_name)
) engine = innodb DEFAULT charset = utf8 comment = 'Quarantine config';


CREATE TABLE IF NOT EXISTS config_listener (
  id bigint(11) NOT NULL AUTO_INCREMENT,
  gmt_created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  gmt_modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  data_id varchar(200) NOT NULL,
  status int NOT NULL COMMENT '0:normal, 1:removed',
  op_version bigint NOT NULL,
  extras varchar(1024) DEFAULT NULL,
  PRIMARY KEY (id),
  INDEX idx_modify_ts (gmt_modified),
  INDEX idx_status (status),
  UNIQUE KEY uk_data_id (data_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8;

create table if not exists inst_config (
  id bigint(11) NOT NULL AUTO_INCREMENT,
  gmt_created timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  gmt_modified timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  inst_id varchar(128) NOT NULL,
  param_key varchar(128) NOT NULL,
  param_val varchar(1024) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_inst_id_key (inst_id, param_key)
) ENGINE = InnoDB DEFAULT CHARSET = utf8;

CREATE TABLE IF NOT EXISTS polardbx_extra (
  id BIGINT(11) NOT NULL auto_increment,
  inst_id VARCHAR(128) NOT NULL,
  name VARCHAR(128) NOT NULL,
  type VARCHAR(10) NOT NULL,
  comment VARCHAR(256) NOT NULL,
  status INT(4) NOT NULL,
  gmt_created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  gmt_modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP on
             UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE uk_inst_id_name_type (inst_id, name, type)
) engine = innodb DEFAULT charset = utf8 COLLATE = utf8_unicode_ci comment = 'extra table for polardbx manager';

CREATE TABLE IF NOT EXISTS schema_change (
    id           BIGINT(11)      NOT NULL AUTO_INCREMENT,
    table_name   varchar(64)     NOT NULL,
    version      int unsigned    NOT NULL,
    gmt_created  timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY table_name (table_name)
    )   ENGINE = innodb DEFAULT CHARSET=utf8;

CREATE TABLE IF NOT EXISTS k8s_topology (
    id BIGINT(11) NOT NULL AUTO_INCREMENT,
    uid VARCHAR(128) NOT NULL,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(10) NOT NULL,
    gmt_created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    gmt_modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY(uid),
    UNIQUE KEY(name, type)
) ENGINE = InnoDB DEFAULT CHARSET = utf8 COLLATE = utf8_unicode_ci COMMENT = 'PolarDBX K8s Topology';

