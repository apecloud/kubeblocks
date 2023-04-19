#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# requires `git` and `gh` commands, ref. https://cli.github.com/manual/installation for installation guides.

. ./gh_env
. ./functions.bash

gh_get_issues "none" "kind/bug"

rows=$(echo ${last_issue_list}| jq -r '. | sort_by(.state,.number)| .[].number')


printf "%s | %s | %s \n" "Issue Title" "Assignees" "Issue State"
echo "---|---|---"
for row in $rows
do 
    issue_id=$(echo $row | awk -F "," '{print $1}')
    gh_get_issue_body ${issue_id}
    printf "[%s](%s) #%s | %s | %s\n" "${last_issue_title}" "${last_issue_url}" "${issue_id}" "${last_issue_assignees_printable}" "${last_issue_state}"

    gh_update_issue_milestone ${issue_id}
done