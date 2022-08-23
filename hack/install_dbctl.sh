#!/usr/bin/env bash

# Implemented based on Dapr Cli https://github.com/dapr/cli/tree/master/install

# dbctl location
: ${DBCTL_INSTALL_DIR:="/usr/local/bin"}
: ${DBCTL_BREW_INSTALL_DIR:="/opt/homebrew/bin"}

# sudo is required to copy binary to DBCTL_INSTALL_DIR for linux
: ${USE_SUDO:="false"}

# Http request CLI
DBCTL_HTTP_REQUEST_CLI=curl

# dbctl filename
DBCTL_CLI_FILENAME=dbctl

DBCTL_CLI_FILE="${DBCTL_INSTALL_DIR}/${DBCTL_CLI_FILENAME}"
DBCTL_CLI_BREW_FILE="${DBCTL_BREW_INSTALL_DIR}/${DBCTL_CLI_FILENAME}"

# created a read-only token to access dbctl packages on jihu.
# should be removed later
DBCTL_CLI_TOKEN_USERNAME="gitlab+deploy-token-3512"
DBCTL_CLI_TOKEN_PASSWORD="wJBLrdCpJRiFsW67k3Y7"
DBCTL_CLI_TOKEN=${DBCTL_CLI_TOKEN_USERNAME}:${DBCTL_CLI_TOKEN_PASSWORD}
DOWNLOAD_BASE="https://${DBCTL_CLI_TOKEN}@jihulab.com/api/v4/projects/40734/packages/generic/infracreate/"

getSystemInfo() {
    ARCH=$(uname -m)
    case $ARCH in
        armv7*) ARCH="arm";;
        aarch64) ARCH="arm64";;
        x86_64) ARCH="amd64";;
    esac

    OS=$(echo `uname`|tr '[:upper:]' '[:lower:]')

    # Most linux distro needs root permission to copy the file to /usr/local/bin
    if [ "$OS" == "linux" ] || [ "$OS" == "darwin" ]; then
        if [ "$DBCTL_INSTALL_DIR" == "/usr/local/bin" ]; then
            USE_SUDO="true"
        fi
    fi
}

verifySupported() {
    local supported=(darwin-amd64 darwin-arm64 linux-amd64 linux-arm linux-arm64)
    local current_osarch="${OS}-${ARCH}"

    for osarch in "${supported[@]}"; do
        if [ "$osarch" == "$current_osarch" ]; then
            echo "Your system is ${OS}_${ARCH}"
            return
        fi
    done

    echo "No prebuilt binary for ${current_osarch}"
    exit 1
}

runAsRoot() {
    local CMD="$*"

    if [ $EUID -ne 0 -a $USE_SUDO = "true" ]; then
        CMD="sudo $CMD"
    fi

    $CMD
}

checkHttpRequestCLI() {
    if type "curl" > /dev/null; then
        DBCTL_HTTP_REQUEST_CLI=curl
    elif type "wget" > /dev/null; then
        DBCTL_HTTP_REQUEST_CLI=wget
    else
        echo "Either curl or wget is required"
        exit 1
    fi
}

checkExisting() {

    if [ -f "$DBCTL_CLI_FILE" ]; then
        echo -e "\ndbctl is detected: $DBCTL_CLI_FILE"
        echo -e "\nPlease uninstall first"
        exit 1
    elif [ -f "$DBCTL_CLI_BREW_FILE" ]; then
        echo -e "\ndbctl is detected: $DBCTL_CLI_BREW_FILE"
        echo -e "\nPlease uninstall first"
        exit 1
    else
        echo -e "Installing dbctl ...\n"
    fi
}

downloadFile() {
    LATEST_RELEASE_TAG=$1

    DBCTL_CLI_ARTIFACT="${DBCTL_CLI_FILENAME}-${OS}-${ARCH}-${LATEST_RELEASE_TAG}.tar.gz"
    # convert `-` to `_` to let it work
    DOWNLOAD_URL="${DOWNLOAD_BASE}/${LATEST_RELEASE_TAG}/${DBCTL_CLI_ARTIFACT}"
    
    # Create the temp directory
    DBCTL_TMP_ROOT=$(mktemp -dt dbctl-install-XXXXXX)
    ARTIFACT_TMP_FILE="$DBCTL_TMP_ROOT/$DBCTL_CLI_ARTIFACT"
    # todo curl with token
    echo "Downloading ..."
    if [ "$DBCTL_HTTP_REQUEST_CLI" == "curl" ]; then
        curl -SL "$DOWNLOAD_URL" -o "$ARTIFACT_TMP_FILE"
    else
        wget -O "$ARTIFACT_TMP_FILE" "$DOWNLOAD_URL"
    fi

    if [ ! -f "$ARTIFACT_TMP_FILE" ]; then
        echo "failed to download $DOWNLOAD_URL ..."
        exit 1
    fi
}

installFile() {
    tar xf "$ARTIFACT_TMP_FILE" -C "$DBCTL_TMP_ROOT"
    local tmp_root_dbctl="$DBCTL_TMP_ROOT/${OS}-${ARCH}/$DBCTL_CLI_FILENAME"
    
    if [ ! -f "$tmp_root_dbctl" ]; then
        echo "Failed to unpack dbctl executable."
        exit 1
    fi

    chmod o+x "$tmp_root_dbctl"
    runAsRoot cp "$tmp_root_dbctl" "$DBCTL_INSTALL_DIR"

    if [ $? -eq 0 ] && [ -f "$DBCTL_CLI_FILE" ]; then
        echo "dbctl installed successfully."
        dbctl --version
        echo -e "Make sure your docker service is running and begin your journey with dbctl:\n"
        echo -e "\t$DBCTL_CLI_FILENAME playground init"
    else
        echo "Failed to install $DBCTL_CLI_FILENAME"
        exit 1
    fi
}

fail_trap() {
    result=$?
    if [ "$result" != "0" ]; then
        echo "Failed to install dbctl"
        echo "Go to https://infracreate.io for more support."
    fi
    cleanup
    exit $result
}

cleanup() {
    if [[ -d "${DBCTL_TMP_ROOT:-}" ]]; then
       rm -rf "$DBCTL_TMP_ROOT"
    fi
}

installCompleted() {
    echo -e "\nFor more information on how to started, please visit:"
    echo "  https://infracreate.io"
}

# -----------------------------------------------------------------------------
# main
# -----------------------------------------------------------------------------
trap "fail_trap" EXIT

getSystemInfo
verifySupported
checkExistingDBCtl
checkHttpRequestCLI


if [ -z "$1" ]; then
    echo "Getting the latest dbctl..."
    ret_val="v0.2.0"
elif [[ $1 == v* ]]; then
    ret_val=$1
else
    ret_val=v$1
fi

downloadFile $ret_val
installFile
cleanup
installCompleted