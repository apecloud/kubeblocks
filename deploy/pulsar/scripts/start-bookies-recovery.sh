#!/bin/sh
bin/apply-config-from-env.py conf/bookkeeper.conf
exec bin/bookkeeper autorecovery