#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# requires `git` and `gh` commands, ref. https://cli.github.com/manual/installation for installation guides.

workdir=$(dirname $0)
. ${workdir}/gh_env
. ${workdir}/functions.bash

process_issue_rows() {
    for ((i = 0; i < ${item_count}; i++))
    do 
        local issue_body=$(echo ${last_issue_list} | jq -r ".[${i}]")
        local issue_id=$(echo ${issue_body} | jq -r ".number")
        local url=$(echo ${issue_body} | jq -r '.html_url')
        local title=$(echo ${issue_body} | jq -r '.title')
        local assignees=$(echo ${issue_body} | jq -r '.assignees[]?.login')
        local state=$(echo ${issue_body}| jq -r '.state')
        printf "[%s](%s) #%s | %s | %s | %s \n" "${title}" "${url}" "${issue_id}" "$(join_by , ${assignees})" "${state}"
        gh_update_issue_milestone ${issue_id}
    done
}

item_count=100
page=1
printf "%s | %s | %s \n" "Issue Title" "Assignees" "Issue State"
echo "---|---|---"
while [ "${item_count}" == "100" ]
do
    gh_get_issues "none" "kind/bug" "open" ${page}
    item_count=$(echo ${last_issue_list} | jq -r '. | length')
    process_issue_rows 
    page=$((page+1))
done