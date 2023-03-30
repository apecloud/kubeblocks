#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import os
import sys
import yaml

from pyjavaproperties import Properties

_PG_DATA = '/home/postgres/pgdata'
_PG_CONF_DIR = _PG_DATA + '/conf'
_PG_HBA_FILE = '/home/postgres/conf/pg_hba.conf'
_PG_CONF_FILE = '/home/postgres/conf/postgresql.conf'
_PG_RECOVERY = '/home/postgres/conf/kb_restore.conf'
_PG_RECOVERY_SIGNAL = 'kb_restore.signal'

_DYNAMIC_PARAMETERS = [
    'archive_mode',
    'archive_timeout',
    'wal_level',
    'wal_log_hints',
    'wal_compression',
    'max_wal_senders',
    'max_connections',
    'max_replication_slots',
    'hot_standby',
    'tcp_keepalives_idle',
    'tcp_keepalives_interval',
    'log_line_prefix',
    'log_checkpoints',
    'log_lock_waits',
    'log_min_duration_statement',
    'log_autovacuum_min_duration',
    'log_connections',
    'log_disconnections',
    'log_statement',
    'log_temp_files',
    'track_functions',
    'checkpoint_completion_target',
    'autovacuum_max_workers',
    'autovacuum_vacuum_scale_factor',
    'autovacuum_analyze_scale_factor',
]

_LOCAL_PARAMETERS = [
    'archive_command',
    'shared_buffers',
    'logging_collector',
    'log_destination',
    'log_directory',
    'log_filename',
    'log_file_mode',
    'log_rotation_age',
    'log_truncate_on_rotation',
    'ssl',
    'ssl_ca_file',
    'ssl_crl_file',
    'ssl_cert_file',
    'ssl_key_file',
    'shared_preload_libraries',
    'bg_mon.listen_address',
    'bg_mon.history_buckets',
    'pg_stat_statements.track_utility',
    'extwlist.extensions',
    'extwlist.custom_path',
]


def _process_pg_parameters(parameters, param_limits):
    return {name: value.strip("'") for name, value in (parameters or {}).items()
            if name in param_limits}


def write_file(config, filename, overwrite):
    if not overwrite and os.path.exists(filename):
        pass
    else:
        with open(filename, 'w') as f:
            f.write(config)


def read_file_lines(file):
    ret = []
    for line in file.readlines():
        line = line.strip()
        if line and not line.startswith('#'):
            ret.append(line)
    return ret


def update_dynamic_config(props, config):
    if not 'parameters' in config:
        config['parameters'] = {}
    config['parameters'].update(_process_pg_parameters(props.getPropertyDict(), _DYNAMIC_PARAMETERS))


def update_local_config(props, config):
    if not 'parameters' in config:
        config['parameters'] = {}
    config['parameters'].update(_process_pg_parameters(props.getPropertyDict(), _LOCAL_PARAMETERS))


def main(filename):
    restore_dir = os.environ.get('RESTORE_DATA_DIR', '')
    local_config = yaml.safe_load(
        os.environ.get('SPILO_CONFIGURATION', os.environ.get('PATRONI_CONFIGURATION', ''))) or {}

    if not 'postgresql' in local_config:
        local_config['postgresql'] = {}

    if not 'bootstrap' in local_config:
        local_config['bootstrap'] = {}

    postgresql = local_config['postgresql']
    postgresql['config_dir'] = _PG_CONF_DIR
    postgresql['custom_conf'] = _PG_CONF_FILE

    # TODO add local postgresql.parameters
    # add pg_hba.conf
    with open(_PG_HBA_FILE, 'r') as f:
        lines = read_file_lines(f)
        if lines:
            postgresql['pg_hba'] = lines

    if restore_dir and os.path.isfile(os.path.join(restore_dir, _PG_RECOVERY_SIGNAL)):
        with open(_PG_RECOVERY, 'r') as f:
            local_config['bootstrap'].update(yaml.safe_load(f))

    props = Properties()
    # parse postgresql.conf
    with open(_PG_CONF_FILE, 'r') as conf:
        props.load(conf)

    if not 'dcs' in local_config['bootstrap']:
        local_config['bootstrap']['dcs'] = {}

    dynamic_config = local_config['bootstrap']['dcs'].get('postgresql', {})

    # update patroni dynamic config to local_config['bootstrap']['dcs']['postgresql']['parameters']
    update_dynamic_config(props, dynamic_config)
    local_config['bootstrap']['dcs']['postgresql'] = dynamic_config

    # update patroni dynamic config to local_config['postgresql']['parameters']
    update_local_config(props, postgresql)

    write_file(yaml.dump(local_config, default_flow_style=False), filename, True)


if __name__ == '__main__':
    main(sys.argv[1])
