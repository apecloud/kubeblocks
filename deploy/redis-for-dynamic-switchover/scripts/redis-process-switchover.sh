#!/bin/sh
set -ex


current_role=$(cat $1)

echo "current pod changed to $current_role"

echo "$@"