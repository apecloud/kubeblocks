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

downloadDockerImage() {
    LATEST_RELEASE_TAG=$1
    # Create the temp directory
    OPENCLI_TMP_ROOT=$(mktemp -dt opencli-install-XXXXXX)
    # pull image and run
    echo -e "Pulling opencli image..."
    docker run --name opencli -d docker.io/infracreate/opencli:${LATEST_RELEASE_TAG} sh &> /dev/null
    # copy opencli to /tmp-xxx/opencli
    docker cp opencli:/opencli.${OS}.${ARCH} ${OPENCLI_TMP_ROOT}/${OPENCLI_CLI_FILENAME} 2>&1 > /dev/null
    # remove docker
    docker rm opencli 2>&1 > /dev/null
}

installFile() {
  local tmp_root_opencli="$OPENCLI_TMP_ROOT/$OPENCLI_CLI_FILENAME"
  
  if [ ! -f "$tmp_root_opencli" ]; then
      echo "Failed to pull opencli."
      exit 1
  fi

  chmod o+x "$tmp_root_opencli"
  runAsRoot cp "$tmp_root_opencli" "$OPENCLI_INSTALL_DIR"

  if [ $? -eq 0 ] && [ -f "$OPENCLI_CLI_FILE" ]; then
      echo "opencli installed successfully."
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

downloadDockerImage $ret_val
installFile
cleanup
installCompleted