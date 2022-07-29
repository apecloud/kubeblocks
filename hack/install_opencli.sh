#!/usr/bin/env bash

# Implemented based on Dapr Cli https://github.com/dapr/cli/tree/master/install

# opencli location
: ${OPENCLI_INSTALL_DIR:="/usr/local/bin"}
: ${OPENCLI_BREW_INSTALL_DIR:="/opt/homebrew/bin"}

# sudo is required to copy binary to OPENCLI_INSTALL_DIR for linux
: ${USE_SUDO:="false"}

# Http request CLI
OPENCLI_HTTP_REQUEST_CLI=curl

# opencli filename
OPENCLI_CLI_FILENAME=opencli

OPENCLI_CLI_FILE="${OPENCLI_INSTALL_DIR}/${OPENCLI_CLI_FILENAME}"
OPENCLI_CLI_BREW_FILE="${OPENCLI_BREW_INSTALL_DIR}/${OPENCLI_CLI_FILENAME}"

# created a read-only token to access opencli packages on jihu.
# should be removed later
OPENCLI_CLI_TOKEN_USERNAME="gitlab+deploy-token-3512"
OPENCLI_CLI_TOKEN_PASSWORD="wJBLrdCpJRiFsW67k3Y7"
OPENCLI_CLI_TOKEN=${OPENCLI_CLI_TOKEN_USERNAME}:${OPENCLI_CLI_TOKEN_PASSWORD}
DOWNLOAD_BASE="https://${OPENCLI_CLI_TOKEN}@jihulab.com/api/v4/projects/40734/packages/generic/infracreate/"

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
        if [ "$OPENCLI_INSTALL_DIR" == "/usr/local/bin" ]; then
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
        OPENCLI_HTTP_REQUEST_CLI=curl
    elif type "wget" > /dev/null; then
        OPENCLI_HTTP_REQUEST_CLI=wget
    else
        echo "Either curl or wget is required"
        exit 1
    fi
}

checkExistingOpenCli() {
    if [ -f "$OPENCLI_CLI_FILE" ]; then
        echo -e "\nopencli is detected: $OPENCLI_CLI_FILE"
        echo -e "\nPlease uninstall first"
        exit 1
    elif [ -f "$OPENCLI_CLI_BREW_FILE" ]; then
        echo -e "\nopencli is detected: $OPENCLI_CLI_BREW_FILE"
        echo -e "\nPlease uninstall first"
        exit 1
    else
        echo -e "Installing opencli ...\n"
    fi
}

downloadFile() {
    LATEST_RELEASE_TAG=$1

    OPENCLI_CLI_ARTIFACT="${OPENCLI_CLI_FILENAME}-${OS}-${ARCH}-${LATEST_RELEASE_TAG}.tar.gz"
    # convert `-` to `_` to let it work
    DOWNLOAD_URL="${DOWNLOAD_BASE}/${LATEST_RELEASE_TAG}/${OPENCLI_CLI_ARTIFACT}"
    
    # Create the temp directory
    OPENCLI_TMP_ROOT=$(mktemp -dt opencli-install-XXXXXX)
    ARTIFACT_TMP_FILE="$OPENCLI_TMP_ROOT/$OPENCLI_CLI_ARTIFACT"
    # todo curl with token
    echo "Downloading ..."
    if [ "$OPENCLI_HTTP_REQUEST_CLI" == "curl" ]; then
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
    tar xf "$ARTIFACT_TMP_FILE" -C "$OPENCLI_TMP_ROOT"
    local tmp_root_opencli="$OPENCLI_TMP_ROOT/${OS}-${ARCH}/$OPENCLI_CLI_FILENAME"
    
    if [ ! -f "$tmp_root_opencli" ]; then
        echo "Failed to unpack opencli executable."
        exit 1
    fi

    chmod o+x "$tmp_root_opencli"
    runAsRoot cp "$tmp_root_opencli" "$OPENCLI_INSTALL_DIR"

    if [ $? -eq 0 ] && [ -f "$OPENCLI_CLI_FILE" ]; then
        echo "opencli installed successfully."
        opencli --version
        echo -e "Make sure your docker service is running and begin your journey with opencli:\n"
        echo -e "\t$OPENCLI_CLI_FILENAME playground init"
    else
        echo "Failed to install $OPENCLI_CLI_FILENAME"
        exit 1
    fi
}

fail_trap() {
    result=$?
    if [ "$result" != "0" ]; then
        echo "Failed to install opencli"
        echo "Go to https://infracreate.io for more support."
    fi
    cleanup
    exit $result
}

cleanup() {
    if [[ -d "${OPENCLI_TMP_ROOT:-}" ]]; then
       rm -rf "$OPENCLI_TMP_ROOT"
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
checkExistingOpenCli
checkHttpRequestCLI


if [ -z "$1" ]; then
    echo "Getting the latest opencli..."
    ret_val="v0.1.0" 
elif [[ $1 == v* ]]; then
    ret_val=$1
else
    ret_val=v$1
fi

downloadFile $ret_val
installFile
cleanup
installCompleted