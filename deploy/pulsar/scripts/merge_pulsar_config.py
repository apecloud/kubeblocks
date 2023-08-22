#!/usr/bin/env python3

import os, sys


def load_config_file(file):
    lines = []  # List of config file lines
    keys = {}  # Map a key to its line number in the file

    # Load conf file
    for line in open(file):
        lines.append(line)
        line = line.strip()
        if not line or line.startswith('#') or '=' not in line:
            continue
        try:
            k, v = line.split('=', 1)
            keys[k.strip()] = len(lines) - 1
        except:
            print("[%s] skip Processing %s" % (file, line))
            sys.exit(-1)
    return lines, keys


def write_config_file(file, lines):
    with open(file, 'w') as f:
        for line in lines:
            f.write(line)


def update_config_file(file, updated_keys, updated_lines):
    lines, keys = load_config_file(file)
    for key in updated_keys:
        updated_line = updated_lines[updated_keys[key]].strip()
        if key not in keys:
            print('[%s] Adding config: %s' % (file, updated_line))
            lines.append('\n%s\n' % updated_line)
        else:
            print('[%s] Updating config %s' % (file, updated_line))
            lines[keys[key]] = '%s\n' % updated_line
    return lines


def merge_config_files(dest_file, src_file):
    lines, keys = load_config_file(src_file)
    if len(keys) == 0:
        return
    write_config_file(dest_file, update_config_file(dest_file, keys, lines))


if len(sys.argv) < 2:
    print('Usage: %s <FILE> <FILE>' % (sys.argv[0]))
    sys.exit(1)

# Always apply env config to env scripts as well
dst_files = sys.argv[1]
src_files = sys.argv[2]

merge_config_files(dst_files, src_files)