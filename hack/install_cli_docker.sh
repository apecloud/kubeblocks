#!/bin/bash
#
# Copyright (C) 2022-2023 ApeCloud Co., Ltd
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
#     http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

: ${CLI_INSTALL_DIR:="/usr/local/bin"}
: ${CLI_BREW_INSTALL_DIR:="/opt/homebrew/bin"}

# sudo is required to copy binary to CLI_INSTALL_DIR for linux
: ${USE_SUDO:="false"}

# Http request CLI
HTTP_REQUEST_CLI=curl

# cli filename
CLI_FILENAME=kbcli

CLI_FILE="${CLI_INSTALL_DIR}/${CLI_FILENAME}"
CLI_BREW_FILE="${CLI_BREW_INSTALL_DIR}/${CLI_FILENAME}"

getSystemInfo() {
    ARCH=$(uname -m)
    case $ARCH in
        armv7*) ARCH="arm";;
        aarch64) ARCH="arm64";;
        x86_64) ARCH="amd64";;
    esac

    OS=$(echo $(uname) | tr '[:upper:]' '[:lower:]')

    # Most linux distro needs root permission to copy the file to /usr/local/bin
    if [ "$OS" == "linux" ] || [ "$OS" == "darwin" ]; then
        if [ "$CLI_INSTALL_DIR" == "/usr/local/bin" ]; then
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
    if type "curl" >/dev/null; then
        HTTP_REQUEST_CLI=curl
    elif type "wget" >/dev/null; then
        HTTP_REQUEST_CLI=wget
    else
        echo "Either curl or wget is required"
        exit 1
    fi
}

checkExistingCli() {
    if [ -f "$CLI_FILE" ]; then
        echo -e "\nkbcli is detected: $CLI_FILE"
        echo -e "\nPlease uninstall first"
        exit 1
    elif [ -f "$CLI_BREW_FILE" ]; then
        echo -e "\nkbcli is detected: $CLI_BREW_FILE"
        echo -e "\nPlease uninstall first"
        exit 1
    else
        echo -e "Installing kbcli ...\n"
    fi
}

downloadDockerImage() {
    LATEST_RELEASE_TAG=$1
    # Create the temp directory
    CLI_TMP_ROOT=$(mktemp -dt kbcli-install-XXXXXX)
    # pull image and run
    echo -e "Pulling kbcli image..."
    docker run --name kbcli -d docker.io/apecloud/kbcli:${LATEST_RELEASE_TAG} sh &>/dev/null
    # copy kbcli to /tmp-xxx/kbcli
    docker cp kbcli:/kbcli.${OS}.${ARCH} ${CLI_TMP_ROOT}/${CLI_FILENAME} 2>&1 >/dev/null
    # remove docker
    docker rm kbcli 2>&1 >/dev/null
}

installFile() {
    local tmp_root_kbcli="$CLI_TMP_ROOT/$CLI_FILENAME"

    if [ ! -f "$tmp_root_kbcli" ]; then
        echo "Failed to pull kbcli."
        exit 1
    fi

    chmod o+x "$tmp_root_kbcli"
    runAsRoot cp "$tmp_root_kbcli" "$CLI_INSTALL_DIR"

    if [ $? -eq 0 ] && [ -f "$CLI_FILE" ]; then
        echo "kbcli installed successfully."
        kbcli version
        echo -e "Make sure your docker service is running and begin your journey with kbcli:\n"
        echo -e "\t$CLI_FILENAME playground init"
    else
        echo "Failed to install $CLI_FILENAME"
        exit 1
    fi
}

fail_trap() {
    result=$?
    if [ "$result" != "0" ]; then
        echo "Failed to install kbcli"
        echo "Go to https://kubeblocks.io for more support."
    fi
    cleanup
    exit $result
}

cleanup() {
    if [[ -d "${CLI_TMP_ROOT:-}" ]]; then
        rm -rf "$CLI_TMP_ROOT"
    fi
}

installCompleted() {
    echo -e "\nFor more information on how to get started, please visit:"
    echo "  https://kubeblocks.io"
}

# -----------------------------------------------------------------------------
# main
# -----------------------------------------------------------------------------
trap "fail_trap" EXIT

getSystemInfo
verifySupported
checkExistingCli
checkHttpRequestCLI

if [ -z "$1" ]; then
    echo "Getting the latest kbcli..."
    ret_val="v0.1.0"
elif [[ $1 == v* ]]; then
    ret_val=$1
else
    ret_val=v$1
fi

downloadDockerImage $ret_val
installFile
cleanup
installCompleted
