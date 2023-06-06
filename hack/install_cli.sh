#!/usr/bin/env bash
#Copyright (C) 2022-2023 ApeCloud Co., Ltd
#
#This file is part of KubeBlocks project
#
#This program is free software: you can redistribute it and/or modify
#it under the terms of the GNU Affero General Public License as published by
#the Free Software Foundation, either version 3 of the License, or
#(at your option) any later version.
#
#This program is distributed in the hope that it will be useful
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU Affero General Public License for more details.
#
#You should have received a copy of the GNU Affero General Public License
#along with this program.  If not, see <http://www.gnu.org/licenses/>.

: ${CLI_INSTALL_DIR:="/usr/local/bin"}
: ${CLI_BREW_INSTALL_DIR:="/opt/homebrew/bin"}

# sudo is required to copy binary to CLI_INSTALL_DIR for linux
: ${USE_SUDO:="false"}

# Http request CLI
HTTP_REQUEST_CLI=curl

# kbcli filename
CLI_FILENAME=kbcli

CLI_FILE="${CLI_INSTALL_DIR}/${CLI_FILENAME}"
CLI_BREW_FILE="${CLI_BREW_INSTALL_DIR}/${CLI_FILENAME}"

TOKEN="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"
REPO="apecloud/kubeblocks"
GITHUB="https://api.github.com"
DOWNLOAD_BASE="https://github.com/${REPO}/releases/download"
FILE="kbcli-darwin-arm64-v0.3.0.tar.gz" # the name of your release asset file, e.g. build.tar.gz

gh_curl() {
    curl -H "Authorization: token $TOKEN" \
         -H "Accept: application/vnd.github.v3.raw" \
         $@
}

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

downloadFile() {
    LATEST_RELEASE_TAG=$1

    if [ "$LATEST_RELEASE_TAG" = "latest" ]; then
        LATEST_RELEASE_TAG=$(gh_curl -s $GITHUB/repos/$REPO/releases/latest | grep '.tag_name' | awk -F: '{print $2}' | sed 's/,//g;s/\"//g;s/ //g')
    fi

    CLI_ARTIFACT="${CLI_FILENAME}-${OS}-${ARCH}-${LATEST_RELEASE_TAG}.tar.gz"
    asset_id=$(gh_curl -s $GITHUB/repos/$REPO/releases/tags/$LATEST_RELEASE_TAG | grep -B 2 "\"name\": \"$CLI_ARTIFACT\"" | grep -w id | awk -F: '{print $2}' | sed 's/,//g;s/\"//g;s/ //g')

    if [ "$asset_id" = "null" ] || [ -z "$asset_id" ]; then
        echo "ERROR: LATEST_RELEASE_TAG not found $LATEST_RELEASE_TAG"
        exit 1
    fi

    # Create the temp directory
    CLI_TMP_ROOT=$(mktemp -dt kbcli-install-XXXXXX)
    ARTIFACT_TMP_FILE="$CLI_TMP_ROOT/$CLI_ARTIFACT"

    echo "Downloading ..."
    DOWNLOAD_ASSET_URL="https://$TOKEN:@api.github.com/repos/$REPO/releases/assets/$asset_id"
    httpstatus=$(curl -SL -q -w "%{http_code}" --header 'Accept:application/octet-stream' "$DOWNLOAD_ASSET_URL" -o "$ARTIFACT_TMP_FILE")

    if [[ "$httpstatus" != "200" || ! -f "$ARTIFACT_TMP_FILE" ]]; then
        echo "Failed to download $CLI_ARTIFACT"
        exit 1
    fi
}

installFile() {
    local tmp_root_kbcli="$CLI_TMP_ROOT/${OS}-${ARCH}/$CLI_FILENAME"
    tar xf "$ARTIFACT_TMP_FILE" -C "$CLI_TMP_ROOT"

    if [[ $? -ne 0 || ! -f "$tmp_root_kbcli" ]]; then
        echo "Failed to unpack kbcli executable."
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
    echo "Getting the latest kbcli ..."
    ret_val="latest"
elif [[ $1 == v* ]]; then
    ret_val=$1
else
    ret_val=v$1
fi

downloadFile $ret_val
installFile
cleanup
installCompleted
