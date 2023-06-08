#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

show_help() {
cat << EOF
Usage: $(basename "$0") <options>

    -h, --help                Display help
    -t, --type                Operation type
                                1) remove v prefix
                                2) replace '-' with '.'
                                3) get release asset upload url
                                4) get latest release tag
                                5) update release latest
                                6) get the ci trigger mode
                                7) check package version
                                8) kill apiserver and etcd
                                9) remove runner
                                10) trigger release
                                11) release message
                                12) send message
                                13) patch release notes
                                14) ignore cover pkgs
    -tn, --tag-name           Release tag name
    -gr, --github-repo        Github Repo
    -gt, --github-token       Github token
    -rn, --runner-name        The runner name
    -bn, --branch-name        The branch name
    -c, --content             The trigger request content
    -bw, --bot-webhook        The bot webhook
    -tt, --trigger-type       The trigger type (e.g. release/package)
    -ru, --run-url            The run url
    -fl, --file               The release notes file
    -ip, --ignore-pkgs        The ignore cover pkgs
EOF
}

GITHUB_API="https://api.github.com"
LATEST_REPO=apecloud/kubeblocks

main() {
    local TYPE
    local TAG_NAME
    local GITHUB_REPO
    local GITHUB_TOKEN
    local TRIGGER_MODE=""
    local RUNNER_NAME=""
    local BRANCH_NAME=""
    local CONTENT=""
    local BOT_WEBHOOK=""
    local TRIGGER_TYPE="release"
    local RELEASE_VERSION=""
    local RUN_URL=""
    local FILE=""
    local IGNORE_PKGS=""

    parse_command_line "$@"

    case $TYPE in
        1)
            echo "${TAG_NAME/v/}"
        ;;
        2)
            echo "${TAG_NAME/-/.}"
        ;;
        3)
            get_upload_url
        ;;
        4)
            get_latest_tag
        ;;
        5)
            update_release_latest
        ;;
        6)
            get_trigger_mode
        ;;
        7)
            check_package_version
        ;;
        8)
            kill_server_etcd
        ;;
        9)
            remove_runner
        ;;
        10)
            trigger_release
        ;;
        11)
            release_message
        ;;
        12)
            send_message
        ;;
        13)
            patch_release_notes
        ;;
        14)
            ignore_cover_pkgs
        ;;
        *)
            show_help
            break
        ;;
    esac
}

parse_command_line() {
    while :; do
        case "${1:-}" in
            -h|--help)
                show_help
                exit
                ;;
            -t|--type)
                if [[ -n "${2:-}" ]]; then
                    TYPE="$2"
                    shift
                fi
                ;;
            -tn|--tag-name)
                if [[ -n "${2:-}" ]]; then
                    TAG_NAME="$2"
                    shift
                fi
                ;;
            -gr|--github-repo)
                if [[ -n "${2:-}" ]]; then
                    GITHUB_REPO="$2"
                    shift
                fi
                ;;
            -gt|--github-token)
                if [[ -n "${2:-}" ]]; then
                    GITHUB_TOKEN="$2"
                    shift
                fi
                ;;
            -rn|--runner-name)
                if [[ -n "${2:-}" ]]; then
                    RUNNER_NAME="$2"
                    shift
                fi
                ;;
            -bn|--branch-name)
                if [[ -n "${2:-}" ]]; then
                    BRANCH_NAME="$2"
                    shift
                fi
                ;;
            -c|--content)
                if [[ -n "${2:-}" ]]; then
                    CONTENT="$2"
                    shift
                fi
                ;;
            -bw|--bot-webhook)
                if [[ -n "${2:-}" ]]; then
                    BOT_WEBHOOK="$2"
                    shift
                fi
                ;;
            -tt|--trigger-type)
                if [[ -n "${2:-}" ]]; then
                    TRIGGER_TYPE="$2"
                    shift
                fi
                ;;
            -ru|--run-url)
                if [[ -n "${2:-}" ]]; then
                    RUN_URL="$2"
                    shift
                fi
                ;;
            -fl|--file)
                if [[ -n "${2:-}" ]]; then
                    FILE="$2"
                    shift
                fi
                ;;
            -ip|--ignore-pkgs)
                if [[ -n "${2:-}" ]]; then
                    IGNORE_PKGS="$2"
                    shift
                fi
                ;;
            *)
                break
                ;;
        esac

        shift
    done
}

gh_curl() {
    curl -H "Authorization: token $GITHUB_TOKEN" \
      -H "Accept: application/vnd.github.v3.raw" \
      $@
}

get_upload_url() {
    gh_curl -s $GITHUB_API/repos/$GITHUB_REPO/releases/tags/$TAG_NAME > release_body.json
    echo $(jq '.upload_url' release_body.json) | sed 's/\"//g'
}

get_latest_tag() {
    latest_release_tag=`gh_curl -s $GITHUB_API/repos/$LATEST_REPO/releases/latest | jq -r '.tag_name'`
    echo $latest_release_tag
}

update_release_latest() {
    release_id=`gh_curl -s $GITHUB_API/repos/$GITHUB_REPO/releases/tags/$TAG_NAME | jq -r '.id'`

    gh_curl -X PATCH \
        $GITHUB_API/repos/$GITHUB_REPO/releases/$release_id \
        -d '{"draft":false,"prerelease":false,"make_latest":true}'
}

kill_server_etcd() {
    server="kube-apiserver\|etcd"
    for pid in $( ps -ef | grep "$server" | grep -v "grep $server" | awk '{print $2}' ); do
        kill $pid
    done
}

remove_runner() {
    runners_url=$GITHUB_API/repos/$LATEST_REPO/actions/runners
    runners_list=$( gh_curl -s $runners_url )
    total_count=$( echo "$runners_list" | jq '.total_count' )
    for i in $(seq 0 $total_count); do
        if [[ "$i" == "$total_count" ]]; then
            break
        fi
        runner_name=$( echo "$runners_list" | jq ".runners[$i].name" --raw-output )
        runner_status=$( echo "$runners_list" | jq ".runners[$i].status" --raw-output )
        runner_busy=$( echo "$runners_list" | jq ".runners[$i].busy" --raw-output )
        runner_id=$( echo "$runners_list" | jq ".runners[$i].id" --raw-output )
        if [[ "$runner_name" == "$RUNNER_NAME" && "$runner_status" == "online" && "$runner_busy" == "false"  ]]; then
            echo "runner_name:"$runner_name
            gh_curl -L -X DELETE $runners_url/$runner_id
            break
        fi
    done
}

check_numeric() {
    input=${1:-""}
    if [[ $input =~ ^[0-9]+$ ]]; then
        echo $(( ${input} ))
    else
        echo "no"
    fi
}

get_next_available_tag() {
    tag_type="$1"
    index=""
    release_list=$( gh release list --repo $LATEST_REPO --limit 100 )
    for tag in $( echo "$release_list" | (grep "$tag_type" || true) ) ;do
        if [[ "$tag" != "$tag_type"* ]]; then
            continue
        fi
        tmp=${tag#*$tag_type}
        numeric=$( check_numeric "$tmp" )
        if [[ "$numeric" == "no" ]]; then
            continue
        fi
        if [[ $numeric -gt $index || -z "$index" ]]; then
            index=$numeric
        fi
    done

    if [[ -z "$index" ]];then
        index=0
    else
        index=$(( $index + 1 ))
    fi

    RELEASE_VERSION="${tag_type}${index}"
}

release_next_available_tag() {
    dispatches_url=$1
    v_major_minor="v$TAG_NAME"
    stable_type="$v_major_minor."
    get_next_available_tag $stable_type
    v_number=$RELEASE_VERSION
    alpha_type="$v_number-alpha."
    beta_type="$v_number-beta."
    rc_type="$v_number-rc."
    case "$CONTENT" in
        *alpha*)
            get_next_available_tag "$alpha_type"
        ;;
        *beta*)
            get_next_available_tag "$beta_type"
        ;;
        *rc*)
            get_next_available_tag "$rc_type"
        ;;
    esac

    if [[ ! -z "$RELEASE_VERSION" ]];then
        gh_curl -X POST $dispatches_url -d '{"ref":"'$BRANCH_NAME'","inputs":{"release_version":"'$RELEASE_VERSION'"}}'
    fi
}

usage_message() {
    curl -H "Content-Type: application/json" -X POST $BOT_WEBHOOK \
        -d '{"msg_type":"post","content":{"post":{"zh_cn":{"title":"Usage:","content":[[{"tag":"text","text":"please enter the correct format\n"},{"tag":"text","text":"1. do <alpha|beta|rc|stable> release\n"},{"tag":"text","text":"2. {\"ref\":\"<ref_branch>\",\"inputs\":{\"release_version\":\"<release_version>\"}}"}]]}}}}'
}

trigger_release() {
    echo "CONTENT:$CONTENT"
    dispatches_url=$GITHUB_API/repos/$LATEST_REPO/actions/workflows/$TRIGGER_TYPE-version.yml/dispatches

    if [[ "$CONTENT" == "do"*"release" ]]; then
        release_next_available_tag "$dispatches_url"
    else
        usage_message
    fi
}

release_message() {
    curl -H "Content-Type: application/json" -X POST $BOT_WEBHOOK \
        -d '{"msg_type":"post","content":{"post":{"zh_cn":{"title":"Release:","content":[[{"tag":"text","text":"yes master, release "},{"tag":"a","text":"['$TAG_NAME']","href":"https://github.com/'$LATEST_REPO'/releases/tag/'$TAG_NAME'"},{"tag":"text","text":" is on its way..."}]]}}}}'
}

send_message() {
    if [[ "$TAG_NAME" != "v"*"."*"."* ]]; then
        echo "invalid tag name"
        return
    fi

    if [[ "$CONTENT" == *"success" ]]; then
        curl -H "Content-Type: application/json" -X POST $BOT_WEBHOOK \
            -d '{"msg_type":"post","content":{"post":{"zh_cn":{"title":"Success:","content":[[{"tag":"text","text":"'$CONTENT'"}]]}}}}'
    else
        curl -H "Content-Type: application/json" -X POST $BOT_WEBHOOK \
            -d '{"msg_type":"post","content":{"post":{"zh_cn":{"title":"Error:","content":[[{"tag":"a","text":"['$CONTENT']","href":"'$RUN_URL'"}]]}}}}'
    fi
}

add_trigger_mode() {
    trigger_mode=$1
    if [[ "$TRIGGER_MODE" != *"$trigger_mode"* ]]; then
        TRIGGER_MODE=$trigger_mode$TRIGGER_MODE
    fi
}

get_trigger_mode() {
    for filePath in $( git diff --name-only HEAD HEAD^ ); do
        if [[ "$filePath" == "go."* ]]; then
            add_trigger_mode "[test]"
            continue
        elif [[ "$filePath" != *"/"* ]]; then
            add_trigger_mode "[other]"
            continue
        fi

        case $filePath in
            docs/*)
                add_trigger_mode "[docs]"
            ;;
            docker/*)
                add_trigger_mode "[docker]"
            ;;
            deploy/*)
                add_trigger_mode "[deploy]"
            ;;
            .github/*|.devcontainer/*|githooks/*|examples/*)
                add_trigger_mode "[other]"
            ;;
            internal/cli/cmd/*)
                add_trigger_mode "[cli][test]"
            ;;
            *)
                add_trigger_mode "[test]"
            ;;
        esac
    done
    echo $TRIGGER_MODE
}

check_package_version() {
    exit_status=0
    beta_tag="v"*"."*"."*"-beta."*
    rc_tag="v"*"."*"."*"-rc."*
    release_tag="v"*"."*"."*
    not_release_tag="v"*"."*"."*"-"*
    if [[ "$TAG_NAME" == $release_tag && "$TAG_NAME" != $not_release_tag ]]; then
        echo "::error title=Release Version Not Allow::$(tput -T xterm setaf 1) $TAG_NAME does not allow packaging.$(tput -T xterm sgr0)"
        exit_status=1
    elif [[ "$TAG_NAME" == $beta_tag ]]; then
        echo "::error title=Beta Version Not Allow::$(tput -T xterm setaf 1) $TAG_NAME does not allow packaging.$(tput -T xterm sgr0)"
        exit_status=1
    elif [[ "$TAG_NAME" == $rc_tag ]]; then
        echo "::error title=Release Candidate Version Not Allow::$(tput -T xterm setaf 1) $TAG_NAME does not allow packaging.$(tput -T xterm sgr0)"
        exit_status=1
    else
        echo "$(tput -T xterm setaf 2)Version allows packaging$(tput -T xterm sgr0)"
    fi
    exit $exit_status
}

patch_release_notes() {
    release_note=""
    while read line; do
      if [[ -z "${release_note}" ]]; then
        release_note="$line"
      else
        release_note="$release_note\n$line"
      fi
    done < ${FILE}

    release_id=`gh_curl -s $GITHUB_API/repos/$GITHUB_REPO/releases/tags/$TAG_NAME | jq -r '.id'`

    curl -H "Authorization: token $GITHUB_TOKEN" \
          -H "Accept: application/vnd.github.v3.raw" \
           -X PATCH \
    $GITHUB_API/repos/$GITHUB_REPO/releases/$release_id \
    -d '{"body":"'"$release_note"'"}'
}

ignore_cover_pkgs() {
    ignore_pkgs=$(echo "$IGNORE_PKGS" | sed 's/|/ /g')
    while read line; do
      ignore=false
      for pkgs in $(echo "$ignore_pkgs"); do
          if [[ "$line" == *"$LATEST_REPO/$pkgs"* ]]; then
              ignore=true
              break
          fi
      done
      if [[ $ignore == true ]]; then
          continue
      fi
      echo $line >> cover_new.out
    done < ${FILE}
}

main "$@"
