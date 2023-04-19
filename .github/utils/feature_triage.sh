#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# requires `git`, `gh`, and `jq` commands, ref. https://cli.github.com/manual/installation for installation guides.

. ./gh_env
. ./functions.bash

gh_get_issues ${MILESTONE_ID} "kind/feature" "all"

rows=$(echo ${last_issue_list}| jq -r '. | sort_by(.state,.number)| .[].number')

echo $rows

printf "%s | %s | %s | %s | %s | %s\n" "Feature Title" "Assignees" "Issue State" "Code PR Merge Status" "Feature Doc. Status" "Extra Notes"
echo "---|---|---|---|---|---"
for row in $rows
do 
    issue_id=$(echo $row | awk -F "," '{print $1}')
    gh_get_issue_body ${issue_id}
    pr_url=$(echo $last_issue_body| jq -r '.pull_request?.url')
    if [ "$pr_url" == "null" ]; then
        pr_url="N/A"
    fi
    printf "[%s](%s) #%s | %s | %s | %s| | \n" "${last_issue_title}" "${last_issue_url}" "${issue_id}" "${last_issue_assignees_printable}" "${last_issue_state}"  "${pr_url}"
done