#!/bin/sh

set -o errexit
set -o nounset
echo "[$(date -Iseconds)] [mount Fix] Evacuating mount --make-rshared / ..."
mount --make-rshared /
echo "[$(date -Iseconds)] [mount Fix] Done"
