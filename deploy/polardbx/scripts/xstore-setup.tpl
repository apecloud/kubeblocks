#!/bin/sh
# usage: xstore-setup.sh
# setup root account for xstore and run entrypoint

function setup_account() {
    until myc -e 'select 1'; do
        sleep 1;
        echo "wait mysql ready"
    done
    echo "mysql is ok"
    myc -e "SET sql_log_bin=OFF;SET force_revise=ON;CREATE USER IF NOT EXISTS $KB_SERVICE_USER IDENTIFIED BY '$KB_SERVICE_PASSWORD';GRANT ALL PRIVILEGES ON *.* TO $KB_SERVICE_USER;ALTER USER $KB_SERVICE_USER IDENTIFIED BY '$KB_SERVICE_PASSWORD';"

}


setup_account &
/tools/xstore/current/venv/bin/python3 /tools/xstore/current/entrypoint.py
