postgresql_set_property() {
    local -r property="${1:?missing property}"
    local -r value="${2:?missing value}"
    local -r conf_file="${3:-$POSTGRESQL_CONF_FILE}"
    local psql_conf
    if grep -qE "^#*\s*${property}" "$conf_file" >/dev/null; then
        replace_in_file "$conf_file" "^#*\s*${property}\s*=.*" "${property} = '${value}'" false
    else
        echo >> "$conf_file"
        echo "${property} = '${value}'" >>"$conf_file"
    fi
}
############
replace_in_file() {
    local filename="${1:?filename is required}"
    local match_regex="${2:?match regex is required}"
    local substitute_regex="${3:?substitute regex is required}"
    local posix_regex=${4:-true}

    local result

    # We should avoid using 'sed in-place' substitutions
    # 1) They are not compatible with files mounted from ConfigMap(s)
    # 2) We found incompatibility issues with Debian10 and "in-place" substitutions
    local -r del=$'\001' # Use a non-printable character as a 'sed' delimiter to avoid issues
    if [[ $posix_regex = true ]]; then
        result="$(sed -E "s${del}${match_regex}${del}${substitute_regex}${del}g" "$filename")"
    else
        result="$(sed "s${del}${match_regex}${del}${substitute_regex}${del}g" "$filename")"
    fi
    echo "$result" > "$filename"
}
############
am_i_root() {
    if [[ "$(id -u)" = "0" ]]; then
        true
    else
        false
    fi
}
############
user_exists() {
    local user="${1:?user is missing}"
    id "$user" >/dev/null 2>&1
}
############
group_exists() {
    local group="${1:?group is missing}"
    getent group "$group" >/dev/null 2>&1
}
############
ensure_group_exists() {
    local group="${1:?group is missing}"
    local gid=""
    local is_system_user=false
    # Validate arguments
    shift 1
    while [ "$#" -gt 0 ]; do
        case "$1" in
        -i | --gid)
            shift
            gid="${1:?missing gid}"
            ;;
        -s | --system)
            is_system_user=true
            ;;
        *)
            echo "Invalid command line flag $1" >&2
            return 1
            ;;
        esac
        shift
    done
    if ! group_exists "$group"; then
        local -a args=("$group")
        if [[ -n "$gid" ]]; then
            if group_exists "$gid"; then
                echo "The GID $gid is already in use." >&2
                return 1
            fi
            args+=("--gid" "$gid")
        fi
        $is_system_user && args+=("--system")
        groupadd "${args[@]}" >/dev/null 2>&1
    fi
}
############
ensure_user_exists() {
    local user="${1:?user is missing}"
    local uid=""
    local group=""
    local append_groups=""
    local home=""
    local is_system_user=false
    # Validate arguments
    shift 1
    while [ "$#" -gt 0 ]; do
        case "$1" in
        -i | --uid)
            shift
            uid="${1:?missing uid}"
            ;;
        -g | --group)
            shift
            group="${1:?missing group}"
            ;;
        -a | --append-groups)
            shift
            append_groups="${1:?missing append_groups}"
            ;;
        -h | --home)
            shift
            home="${1:?missing home directory}"
            ;;
        -s | --system)
            is_system_user=true
            ;;
        *)
            echo "Invalid command line flag $1" >&2
            return 1
            ;;
        esac
        shift
    done
    if ! user_exists "$user"; then
        local -a user_args=("-N" "$user")
        if [[ -n "$uid" ]]; then
            if user_exists "$uid"; then
                echo "The UID $uid is already in use."
                return 1
            fi
            user_args+=("--uid" "$uid")
        else
            $is_system_user && user_args+=("--system")
        fi
        useradd "${user_args[@]}" >/dev/null 2>&1
    fi
    if [[ -n "$group" ]]; then
        local -a group_args=("$group")
        $is_system_user && group_args+=("--system")
        ensure_group_exists "${group_args[@]}"
        usermod -g "$group" "$user" >/dev/null 2>&1
    fi
    if [[ -n "$append_groups" ]]; then
        local -a groups
        read -ra groups <<<"$(tr ',;' ' ' <<<"$append_groups")"
        for group in "${groups[@]}"; do
            ensure_group_exists "$group"
            usermod -aG "$group" "$user" >/dev/null 2>&1
        done
    fi
    if [[ -n "$home" ]]; then
        mkdir -p "$home"
        usermod -d "$home" "$user" >/dev/null 2>&1
        configure_permissions_ownership "$home" -d "775" -f "664" -u "$user" -g "$group"
    fi
}
############
configure_permissions_ownership() {
    local -r paths="${1:?paths is missing}"
    local dir_mode=""
    local file_mode=""
    local user=""
    local group=""

    # Validate arguments
    shift 1
    while [ "$#" -gt 0 ]; do
        case "$1" in
        -f | --file-mode)
            shift
            file_mode="${1:?missing mode for files}"
            ;;
        -d | --dir-mode)
            shift
            dir_mode="${1:?missing mode for directories}"
            ;;
        -u | --user)
            shift
            user="${1:?missing user}"
            ;;
        -g | --group)
            shift
            group="${1:?missing group}"
            ;;
        *)
            echo "Invalid command line flag $1" >&2
            return 1
            ;;
        esac
        shift
    done

    read -r -a filepaths <<<"$paths"
    for p in "${filepaths[@]}"; do
        if [[ -e "$p" ]]; then
            find -L "$p" -printf ""
            if [[ -n $dir_mode ]]; then
                find -L "$p" -type d ! -perm "$dir_mode" -print0 | xargs -r -0 chmod "$dir_mode"
            fi
            if [[ -n $file_mode ]]; then
                find -L "$p" -type f ! -perm "$file_mode" -print0 | xargs -r -0 chmod "$file_mode"
            fi
            if [[ -n $user ]] && [[ -n $group ]]; then
                find -L "$p" -print0 | xargs -r -0 chown "${user}:${group}"
            elif [[ -n $user ]] && [[ -z $group ]]; then
                find -L "$p" -print0 | xargs -r -0 chown "${user}"
            elif [[ -z $user ]] && [[ -n $group ]]; then
                find -L "$p" -print0 | xargs -r -0 chgrp "${group}"
            fi
        else
            echo "$p does not exist"
        fi
    done
}
############
postgresql_slave_init_db() {
    local -r check_args=("-U" "$POSTGRES_USER" "-h" "$POSTGRESQL_MASTER_HOST" "-p" "$POSTGRESQL_MASTER_PORT_NUMBER" "-d" "postgres")
    local check_cmd=()
    if am_i_root; then
        check_cmd=("gosu" "$POSTGRES_USER")
    fi
    check_cmd+=("$POSTGRESQL_BIN_DIR"/pg_isready)
    local ready_counter=$POSTGRESQL_INIT_MAX_TIMEOUT

    while ! PGPASSWORD=$POSTGRES_PASSWORD "${check_cmd[@]}" "${check_args[@]}"; do
        sleep 1
        ready_counter=$((ready_counter - 1))
        if ((ready_counter <= 0)); then
            echo "PostgreSQL master is not ready after $POSTGRESQL_INIT_MAX_TIMEOUT seconds"
            exit 1
        fi

    done
    local -r backup_args=("-D" "$PGDATA" "-U" "$POSTGRES_USER" "-h" "$POSTGRESQL_MASTER_HOST" "-p" "$POSTGRESQL_MASTER_PORT_NUMBER" "-X" "stream" "-w" "-v" "-P")
    local backup_cmd=()
    if am_i_root; then
        backup_cmd+=("gosu" "$POSTGRES_USER")
    fi
    backup_cmd+=("$POSTGRESQL_BIN_DIR"/pg_basebackup)
    local replication_counter=$POSTGRESQL_INIT_MAX_TIMEOUT
    while ! PGPASSWORD=$POSTGRES_PASSWORD "${backup_cmd[@]}" "${backup_args[@]}"; do
        sleep 1
        replication_counter=$((replication_counter - 1))
        if ((replication_counter <= 0)); then
            echo "Slave replication failed after trying for $POSTGRESQL_INIT_MAX_TIMEOUT seconds"
            exit 1
        fi
    done
}