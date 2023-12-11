#!/usr/bin/env bash

#
# Copyright (c) 2023 OceanBase
# ob-operator is licensed under Mulan PSL v2.
# You can use this software according to the terms and conditions of the Mulan PSL v2.
# You may obtain a copy of Mulan PSL v2 at:
#          http://license.coscl.org.cn/MulanPSL2
# THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
# EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
# MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
# See the Mulan PSL v2 for more details.
#

function conn_local {
  mysql -h127.0.0.1 -uroot -P 2881 -A -e "$1"
}

function conn_local_as_tenant {
  mysql -h127.0.0.1 -uroot@"$1" -P 2881 -A -Doceanbase -e "$2"
}

function conn_local_obdb {
  mysql -h127.0.0.1 -uroot -P 2881 -A -Doceanbase -e "$1"
}

function conn_remote {
  mysql -h$1 -uroot -P 2881 -A -e "$2"
}

function conn_remote_obdb {
  mysql -h$1 -uroot -P 2881 -A -Doceanbase -e "$2"
}

function conn_remote_batch {
  # Used for querying results
  mysql -h$1 -uroot -P 2881 -A -Doceanbase -e "$2" -B
}

function conn_remote_as_tenant {
  mysql -h$1 -uroot@"$2" -P 2881 -A -e "$3"
}
