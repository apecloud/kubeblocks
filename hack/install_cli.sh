# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#!/usr/bin/env bash

# kbcli location
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

REPO="apecloud/kbcli"
GITHUB="https://api.github.com"
GITLAB_REPO="85948"
GITLAB="https://jihulab.com/api/v4/projects"
COUNTRY_CODE=""

getCountryCode() {
    COUNTRY_CODE=`curl -m 20 --connect-timeout 10 -s https://ifconfig.io/country_code`
}

getSystemInfo() {
    ARCH=$(uname -m)
    case $ARCH in
        armv7*) ARCH="arm" ;;
        aarch64) ARCH="arm64" ;;
        x86_64) ARCH="amd64" ;;
    esac

    OS=$(echo $(uname) | tr '[:upper:]' '[:lower:]')

    # Most linux distro needs root permission to copy the file to /usr/local/bin
    if [[ "$OS" == "linux" || "$OS" == "darwin" ]] && [ "$CLI_INSTALL_DIR" == "/usr/local/bin" ]; then
        USE_SUDO="true"
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

checkExistingCLI() {
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

getLatestRelease() {
    latest_release=""
    if [[ "$COUNTRY_CODE" == "CN" || "$COUNTRY_CODE" == "" ]]; then
        releaseURL=$GITLAB/$GITLAB_REPO/repository/tags/latest
        if [ "$HTTP_REQUEST_CLI" == "curl" ]; then
            latest_release=`curl -s $releaseURL | grep 'message'|awk 'NR==1{print $1}'`
        else
            latest_release=`wget -q --header="Accept: application/json" -O - $releaseURL | grep 'tag_name'|awk 'NR==1{print $1}'`
        fi
        latest_release=${latest_release#*"message\":\""}
        latest_release=${latest_release%%"\","*}
    else
        releaseURL=$GITHUB/repos/$REPO/releases/latest
        if [ "$HTTP_REQUEST_CLI" == "curl" ]; then
            latest_release=$(curl -s $releaseURL | grep \"tag_name\" | awk 'NR==1{print $2}' | sed -n 's/\"\(.*\)\",/\1/p')
        else
            latest_release=$(wget -q --header="Accept: application/json" -O - $releaseURL | grep \"tag_name\" | awk 'NR==1{print $2}' | sed -n 's/\"\(.*\)\",/\1/p')
        fi
    fi
    ret_val=$latest_release
}

downloadFile() { # for version >= 0.5
    LATEST_RELEASE_TAG=$1

    CLI_ARTIFACT="${CLI_FILENAME}-${OS}-${ARCH}.tar.gz"
    DOWNLOAD_BASE="https://github.com/$REPO/releases/download"
    if [[ "$COUNTRY_CODE" == "CN" || "$COUNTRY_CODE" == "" ]]; then
        DOWNLOAD_BASE="$GITLAB/$GITLAB_REPO/packages/generic/kubeblocks"
    fi
    DOWNLOAD_URL="${DOWNLOAD_BASE}/${LATEST_RELEASE_TAG}/${CLI_ARTIFACT}"

    # Create the temp directory
    CLI_TMP_ROOT=$(mktemp -dt kbcli-install-XXXXXX)
    ARTIFACT_TMP_FILE="$CLI_TMP_ROOT/$CLI_ARTIFACT"

    echo "Downloading ..."
    if [ "$HTTP_REQUEST_CLI" == "curl" ]; then
        curl -SL --header 'Accept:application/octet-stream' "$DOWNLOAD_URL" -o "$ARTIFACT_TMP_FILE"
    else
        wget -q --show-progress -O "$ARTIFACT_TMP_FILE" "$DOWNLOAD_URL"
    fi

    if [[ $? -ne 0 || ! -f "$ARTIFACT_TMP_FILE" ]]; then
        echo "Failed to download $CLI_ARTIFACT."
        exit 1
    fi
}

installFile() { # for version >= 0.5
    LATEST_RELEASE_TAG=$1
    local tmp_root_kbcli="$CLI_TMP_ROOT/${CLI_FILENAME}-${OS}-${ARCH}-${LATEST_RELEASE_TAG}/$CLI_FILENAME"
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
        echo -e ""
    else
        echo "Failed to install $CLI_FILENAME"
        exit 1
    fi
}

downloadOldFile() { # for verion < 0.5.0
    LATEST_RELEASE_TAG=$1

    CLI_ARTIFACT="${CLI_FILENAME}-${OS}-${ARCH}-${LATEST_RELEASE_TAG}.tar.gz"
    DOWNLOAD_BASE="https://github.com/$REPO/releases/download"
    if [[ "$COUNTRY_CODE" == "CN" || "$COUNTRY_CODE" == "" ]]; then
        DOWNLOAD_BASE="$GITLAB/$GITLAB_REPO/packages/generic/kubeblocks"
    fi
    DOWNLOAD_URL="${DOWNLOAD_BASE}/${LATEST_RELEASE_TAG}/${CLI_ARTIFACT}"

    # Create the temp directory
    CLI_TMP_ROOT=$(mktemp -dt kbcli-install-XXXXXX)
    ARTIFACT_TMP_FILE="$CLI_TMP_ROOT/$CLI_ARTIFACT"

    echo "Downloading ..."
    if [ "$HTTP_REQUEST_CLI" == "curl" ]; then
        curl -SL --header 'Accept:application/octet-stream' "$DOWNLOAD_URL" -o "$ARTIFACT_TMP_FILE"
    else
        wget -q --show-progress -O "$ARTIFACT_TMP_FILE" "$DOWNLOAD_URL"
    fi

    if [[ $? -ne 0 || ! -f "$ARTIFACT_TMP_FILE" ]]; then
        echo "Failed to download $CLI_ARTIFACT."
        exit 1
    fi
}

installOldFile() {  # for verion < 0.5.0
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
        echo -e ""
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
checkExistingCLI
checkHttpRequestCLI
getCountryCode

if [ -z "$1" ]; then
    echo "Getting the latest kbcli ..."
    getLatestRelease
elif [[ $1 == v* ]]; then
    ret_val=$1
else
    ret_val=v$1
fi

if [[ "$(printf '%s\n' "$ret_val" "v0.5.0" | sort -V | head -n1)" == "v0.5.0" ]]; then
    # The first element of the sorted result is "v0.5.0", which means that the current version >= "v0.5.0"
    downloadFile $ret_val
    installFile $ret_val
else
    downloadOldFile $ret_val
    installOldFile
fi

cleanup
installCompleted