set -e
mkdir -p ${DATA_DIR}
res=`ls -A ${DATA_DIR}`
data_protection_file=${DATA_DIR}/.kb-data-protection
if [ ! -z "${res}" ] && [ ! -f ${data_protection_file} ]; then
  echo "${DATA_DIR} is not empty! Please make sure that the directory is empty before restoring the backup."
  exit 1
fi
cd ${DATA_DIR} && touch mongodb.backup
touch ${data_protection_file}
tar -xvf ${DP_BACKUP_DIR}/${DP_BACKUP_NAME}.tar.gz -C ${DATA_DIR}
rm -rf ${data_protection_file} && sync