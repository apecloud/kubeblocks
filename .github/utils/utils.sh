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
                                15) set size label
                                16) get test packages
                                17) delete actions cache
                                18) check release tag
    -tn, --tag-name           Release tag name
    -gr, --github-repo        Github Repo
    -gt, --github-token       Github token
    -rn, --runner-name        The runner name
    -bn, --branch-name        The branch name
    -c, --content             The trigger request content
    -bw, --bot-webhook        The bot webhook
    -tt, --trigger-type       The trigger type
    -ru, --run-url            The run url
    -fl, --file               The release notes file
    -ip, --ignore-pkgs        The ignore cover pkgs
    -br, --base-branch        The base branch name
    -bc, --base-commit        The base commit id
    -pn, --pr-number          The pull request number
    -tp, --test-pkgs          The test packages
    -tc, --test-check         The test check
EOF
}

GITHUB_API="https://api.github.com"
LATEST_REPO=apecloud/kubeblocks

main() {
    local TYPE=""
    local TAG_NAME=""
    local GITHUB_REPO=""
    local GITHUB_TOKEN=""
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
    local BASE_BRANCH=""
    local BASE_COMMIT=""
    local BASE_COMMIT_ID=HEAD^
    local PR_NUMBER=""
    local TEST_PACKAGES=""
    local TEST_PKGS=""
    local TEST_CHECK=""

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
        15)
            set_size_label
        ;;
        16)
            get_test_packages
        ;;
        17)
            delete_actions_cache
        ;;
        18)
            check_release_tag
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
            -br|--base-branch)
                if [[ -n "${2:-}" ]]; then
                    BASE_BRANCH="$2"
                    shift
                fi
                ;;
            -bc|--base-commit)
                if [[ -n "${2:-}" ]]; then
                    BASE_COMMIT="$2"
                    shift
                fi
                ;;
            -pn|--pr-number)
                if [[ -n "${2:-}" ]]; then
                    PR_NUMBER="$2"
                    shift
                fi
                ;;
            -tp|--test-pkgs)
                if [[ -n "${2:-}" ]]; then
                    TEST_PKGS="$2"
                    shift
                fi
                ;;
            -tc|--test-check)
                if [[ -n "${2:-}" ]]; then
                    TEST_CHECK="$2"
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
    if [[ -z "$GITHUB_TOKEN" ]]; then
        curl -H "Accept: application/vnd.github.v3.raw" \
            $@
    else
        curl -H "Authorization: token $GITHUB_TOKEN" \
            -H "Accept: application/vnd.github.v3.raw" \
            $@
    fi
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

check_release_version(){
    TMP_TAG_NAME=""
    for content in $(echo "$CONTENT"); do
        if [[ "$content" == "v"*"."* || "$content" == *"."* ]]; then
            TMP_TAG_NAME=$content
        fi
        if [[ -n "$TMP_TAG_NAME" ]]; then
            TMP_BRANCH_NAME="release-${TMP_TAG_NAME/v/}"
            branch_url=$GITHUB_API/repos/$LATEST_REPO/branches/$TMP_BRANCH_NAME
            branch_info=$( gh_curl -s $branch_url | (grep  $TMP_BRANCH_NAME || true) )
            if [[ -n "$branch_info" ]]; then
                BRANCH_NAME=$TMP_BRANCH_NAME
                TAG_NAME=$TMP_TAG_NAME
            fi
            break
        fi
    done
}

release_next_available_tag() {
    check_release_version
    dispatches_url=$1
    v_major_minor="$TAG_NAME"
    if [[ "$TAG_NAME" != "v"* ]]; then
        v_major_minor="v$TAG_NAME"
    fi
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
        -d '{"msg_type":"post","content":{"post":{"zh_cn":{"title":"Usage:","content":[[{"tag":"text","text":"please enter the correct format\n"},{"tag":"text","text":"1. do [v*.*] <alpha|beta|rc> release\n"},{"tag":"text","text":"2. {\"ref\":\"<ref_branch>\",\"inputs\":{\"release_version\":\"<release_version>\"}}"}]]}}}}'
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

get_base_commit_id() {
    if [[ ! -z "$BASE_COMMIT" ]]; then
        BASE_COMMIT_ID=$BASE_COMMIT
        return
    fi
    base_branch_commits="$( git rev-list $BASE_BRANCH -n 100 )"
    current_branch_commits="$( git rev-list $BRANCH_NAME -n 50 )"
    for base_commit_id in $( echo "$base_branch_commits" ); do
        found=false
        for cur_commit_id in $( echo "$current_branch_commits" ); do
            if [[ "$cur_commit_id" == "$base_commit_id" ]]; then
                BASE_COMMIT_ID=$base_commit_id
                found=true
                break
              fi
        done
        if [[ $found == true ]]; then
            break
        fi
    done
}

get_trigger_mode() {
    if [[ ! ("$BRANCH_NAME" == "main" || "$BRANCH_NAME" == "release-"* || "$BRANCH_NAME" == "releasing-"*) ]]; then
        get_base_commit_id
    fi
    echo "BASE_COMMIT_ID:$BASE_COMMIT_ID"
    filePaths=$( git diff --name-only HEAD ${BASE_COMMIT_ID} )
    for filePath in $( echo "$filePaths" ); do
        if [[ "$filePath" == "go."* || "$filePath" == *".go" ]]; then
            add_trigger_mode "[test][go]"
        elif [[ "$filePath" != *"/"* ]]; then
            add_trigger_mode "[other]"
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

set_size_label() {
    pr_info=$( gh pr view $PR_NUMBER --repo $LATEST_REPO --json "additions,deletions,labels" )
    pr_additions=$( echo "$pr_info" | jq -r '.additions' )
    pr_deletions=$( echo "$pr_info" | jq -r '.deletions' )
    total_changes=$(( $pr_additions + $pr_deletions ))
    size_label=""
    if [[ $total_changes -lt 10 ]]; then
        size_label="size/XS"
    elif [[ $total_changes -lt 30 ]]; then
        size_label="size/S"
    elif [[ $total_changes -lt 100 ]]; then
        size_label="size/M"
    elif [[ $total_changes -lt 500 ]]; then
        size_label="size/L"
    elif [[ $total_changes -lt 1000 ]]; then
        size_label="size/XL"
    else
        size_label="size/XXL"
    fi
    echo "size label:$size_label"
    label_list=$(  echo "$pr_info" | jq -r '.labels[].name' )
    remove_label=""
    add_label=true
    for label in $( echo "$label_list" ); do
        case $label in
            $size_label)
                add_label=false
                continue
            ;;
            size/*)
                if [[ -z "$remove_label" ]]; then
                    remove_label=$label
                else
                    remove_label="$label,$remove_label"
                fi
            ;;
        esac
    done

    if [[ ! -z "$remove_label" ]]; then
        echo "remove label:$remove_label"
        gh pr edit $PR_NUMBER --repo $LATEST_REPO --remove-label "$remove_label"
    fi

    if [[ $add_label == true ]]; then
        echo "add label:$size_label"
        gh pr edit $PR_NUMBER --repo $LATEST_REPO --add-label "$size_label"
    fi
}

set_test_packages() {
    pkgs_dir=$1
    if ( find $pkgs_dir -maxdepth 1 -type f -name '*_test.go' ) > /dev/null; then
        if [[ -z "$TEST_PACKAGES" ]]; then
            TEST_PACKAGES="{\"ops\":\"$pkgs_dir\"}"
        else
            TEST_PACKAGES="$TEST_PACKAGES,{\"ops\":\"$pkgs_dir\"}"
        fi
    fi
}

set_test_check() {
    check=$1
    if [[ -z "$TEST_PACKAGES" ]]; then
        TEST_PACKAGES="{\"ops\":\"$check\"}"
    else
        TEST_PACKAGES="$TEST_PACKAGES,{\"ops\":\"$check\"}"
    fi
}

get_test_packages() {
    if [[ "$TRIGGER_TYPE" != *"[test]"* ]]; then
        echo $TEST_PACKAGES
        return
    fi
    for check in $( echo "$TEST_CHECK" | sed 's/|/ /g' ); do
        set_test_check $check
    done

    for pkgs in $( echo "$TEST_PKGS" | sed 's/|/ /g' ); do
        for pkgs_dir in $( find $pkgs -maxdepth 1 -type d ) ; do
            if [[ "$pkgs" == "$pkgs_dir" ]]; then
                continue
            fi
            set_test_packages $pkgs_dir
        done
    done
    echo $TEST_PACKAGES
}

delete_actions_cache() {
    gh extension install actions/gh-actions-cache --force

    gh actions-cache delete --repo $LATEST_REPO $TAG_NAME --confirm
}

check_release_tag(){
    if [[ "$TAG_NAME" == "latest" ]]; then
        echo "$TAG_NAME"
        return
    fi
    release_list=$( gh release list --repo $LATEST_REPO --limit 100 )
    for tag in $( echo "$release_list"); do
        if [[ "$tag" == "$TAG_NAME" ]]; then
            echo "$TAG_NAME"
            break
        elif [[ "$tag" == "v$TAG_NAME" ]]; then
            echo "v$TAG_NAME"
            break
        fi
    done
}

main "$@"
