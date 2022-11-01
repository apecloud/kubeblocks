#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

DEFAULT_CHART_RELEASER_VERSION=v1.4.1

show_help() {
cat << EOF
Usage: $(basename "$0") <options>

    -h, --help               Display help
    -v, --version            The chart-releaser version to use (default: $DEFAULT_CHART_RELEASER_VERSION)"
    -o, --owner              The repo owner
    -r, --repo               The repo name
    -n, --install-dir        The Path to install the cr tool
EOF
}

main() {
    local version="$DEFAULT_CHART_RELEASER_VERSION"
    local owner=
    local repo=
    local install_dir=

    parse_command_line "$@"

    : "${CR_TOKEN:?Environment variable CR_TOKEN must be set}"

    echo 'Adding install_dir to PATH...'
    export PATH="$install_dir:$PATH"

    if [ -d ../.cr-release-packages ]; then
        mv ../.cr-release-packages .
        mv ../.cr-index .

        release_charts
        update_index
    fi
}

parse_command_line() {
    while :; do
        case "${1:-}" in
            -h|--help)
                show_help
                exit
                ;;
            -v|--version)
                if [[ -n "${2:-}" ]]; then
                    version="$2"
                    shift
                else
                    echo "ERROR: '-v|--version' cannot be empty." >&2
                    show_help
                    exit 1
                fi
                ;;
            -o|--owner)
                if [[ -n "${2:-}" ]]; then
                    owner="$2"
                    shift
                else
                    echo "ERROR: '--owner' cannot be empty." >&2
                    show_help
                    exit 1
                fi
                ;;
            -r|--repo)
                if [[ -n "${2:-}" ]]; then
                    repo="$2"
                    shift
                else
                    echo "ERROR: '--repo' cannot be empty." >&2
                    show_help
                    exit 1
                fi
                ;;
            -n|--install-dir)
                if [[ -n "${2:-}" ]]; then
                    install_dir="$2"
                    shift
                fi
                ;;
            *)
                break
                ;;
        esac

        shift
    done

    if [[ -z "$owner" ]]; then
        echo "ERROR: '-o|--owner' is required." >&2
        show_help
        exit 1
    fi

    if [[ -z "$repo" ]]; then
        echo "ERROR: '-r|--repo' is required." >&2
        show_help
        exit 1
    fi

    if [[ -z "$install_dir" ]]; then
        local arch
        arch=$(uname -m)
        install_dir="$RUNNER_TOOL_CACHE/cr/$version/$arch"
    fi
}

release_charts() {
    local args=(-o "$owner" -r "$repo" -c "$(git rev-parse HEAD)" -t $CR_TOKEN --skip-existing)

    echo 'Releasing charts...'
    cr upload "${args[@]}"
}

update_index() {
    local args=(-o "$owner" -r "$repo" -t $CR_TOKEN --push)

    echo 'Updating charts repo index...'
    cr index "${args[@]}"
}

main "$@"
