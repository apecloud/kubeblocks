#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# requires `git` and `gh` commands, ref. https://cli.github.com/manual/installation for installation guides.

workdir=$(dirname $0)
. ${workdir}/gh_env
. ${workdir}/functions.bash

echo "Merging ${PR_TITLE}"


pr_info=$(gh pr list --repo ${OWNER}/${REPO} --base ${BASE_BRANCH} --json "number,url,mergeStateStatus,mergeable" )
pr_number=$(echo ${pr_info} | jq -r '.[0].number') 
pr_merge_status=$(echo ${pr_info} | jq -r '.[0].mergeStateStatus') 
pr_mergeable=$(echo ${pr_info} | jq -r '.[0].mergeable') 

echo "pr_number=${pr_number}"
echo "pr_merge_status=${pr_merge_status}"
echo "pr_mergeable=${pr_mergeable}"

if [ "${pr_merge_status}" == "CLEAN" ] && [ "${pr_mergeable}" == "MERGEABLE" ]; then
    gh pr --repo apecloud/kubeblocks merge ${pr_number} --merge
fi