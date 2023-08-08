#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import os
import sys
import yaml
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
def postgresql_conf_to_dict(file_path):
    with open(file_path, 'r') as f:
        content = f.read()
    lines = content.splitlines()
    result = {}
    for line in lines:
        if line.startswith('#'):
            continue
        if '=' not in line:
            continue
        key, value = line.split('=', 1)
        result[key.strip()] = value.strip().strip("'")
    return result
def main(filename):
    restore_dir = os.environ.get('RESTORE_DATA_DIR', '')
    local_config = yaml.safe_load(
        os.environ.get('SPILO_CONFIGURATION', os.environ.get('PATRONI_CONFIGURATION', ''))) or {}
    if not 'postgresql' in local_config:
        local_config['postgresql'] = {}
    postgresql = local_config['postgresql']
    postgresql['config_dir'] = '/home/postgres/pgdata/conf'
    postgresql['custom_conf'] = '/home/postgres/conf/postgresql.conf'
    # add pg_hba.conf
    with open('/home/postgres/conf/pg_hba.conf', 'r') as f:
        lines = read_file_lines(f)
        if lines:
            postgresql['pg_hba'] = lines
    if restore_dir and os.path.isfile(os.path.join(restore_dir, 'kb_restore.signal')):
        if not 'bootstrap' in local_config:
            local_config['bootstrap'] = {}
        with open('/home/postgres/conf/kb_restore.conf', 'r') as f:
            local_config['bootstrap'].update(yaml.safe_load(f))
    # point in time recovery(PITR)
    data_dir = os.environ.get('PGDATA', '')
    if os.path.isfile("/home/postgres/pgdata/conf/recovery.conf"):
        with open('/home/postgres/conf/kb_pitr.conf', 'r') as f:
            pitr_config = yaml.safe_load(f)
            re_config = postgresql_conf_to_dict("/home/postgres/pgdata/conf/recovery.conf")
            pitr_config[pitr_config['method']]['recovery_conf'].update(re_config)
            local_config['bootstrap'].update(pitr_config)
    write_file(yaml.dump(local_config, default_flow_style=False), filename, True)
if __name__ == '__main__':
    main(sys.argv[1])
