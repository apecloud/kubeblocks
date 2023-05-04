#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# requires `git` and `gh` commands, ref. https://cli.github.com/manual/installation for installation guides.

workdir=$(dirname $0)
. ${workdir}/gh_env
. ${workdir}/functions.bash

get_pr_status() {
    pr_info=$(gh pr --repo ${OWNER}/${REPO} view ${pr_number} --json "mergeStateStatus,mergeable")
    pr_merge_status=$(echo ${pr_info} | jq -r '.mergeStateStatus') 
    pr_mergeable=$(echo ${pr_info} | jq -r '.mergeable') 
    if [ -n "$DEBUG" ]; then
    echo "pr_number=${pr_number}"
    echo "pr_merge_status=${pr_merge_status}"
    echo "pr_mergeable=${pr_mergeable}"
    fi
}

echo "Merging ${PR_TITLE}"

retry_times=0
pr_info=$(gh pr list --repo ${OWNER}/${REPO} --head ${HEAD_BRANCH} --base ${BASE_BRANCH} --json "number" )
pr_len=$(echo ${pr_info}  | jq -r '. | length')
if [ "${pr_len}" == "0" ]; then
exit 0
fi

pr_number=$(echo ${pr_info} | jq -r '.[0].number') 
get_pr_status

if [ "${pr_mergeable}" == "MERGEABLE" ]; then
    if [ "${pr_merge_status}" == "BLOCKED" ]; then
            echo "Approve PR #${pr_number}"
            gh pr --repo ${OWNER}/${REPO} comment ${pr_number} --body "/approve"
            sleep 5
            get_pr_status
    fi

    if [ "${pr_merge_status}" == "UNSTABLE" ]; then
        retry_times=100
        while [ $retry_times -gt 0 ] && [ "${pr_merge_status}" == "UNSTABLE" ]
        do
            ((retry_times--))
            sleep 5
            get_pr_status
        done
    fi

    if [ "${pr_merge_status}" == "CLEAN" ]; then 
        echo "Merging PR #${pr_number}"
        set -x
        gh pr --repo ${OWNER}/${REPO} merge ${pr_number} --rebase
        exit 0
    fi
fi
exit 1

