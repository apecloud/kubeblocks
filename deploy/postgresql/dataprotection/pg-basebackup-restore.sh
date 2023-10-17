set -e
set -o pipefail
export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"
mkdir -p ${DATA_DIR};
datasafed pull "${DP_BACKUP_NAME}.tar.gz" - | gunzip | tar -xvf - -C "${DATA_DIR}/"
echo "done!";