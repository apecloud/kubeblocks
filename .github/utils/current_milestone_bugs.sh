#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# requires `git` and `gh` commands, ref. https://cli.github.com/manual/installation for installation guides.

workdir=$(dirname $0)
. ${workdir}/gh_env
. ${workdir}/functions.bash

LABELS=${LABELS:-'kind/bug,bug'} #"severity/critical,severity/major,severity/minor,severity/normal" 

print_issue_rows() {
    for ((i = 0; i < ${item_count}; i++))
    do 
        local issue_body=$(echo ${last_issue_list} | jq -r ".[${i}]")
        local issue_id=$(echo ${issue_body} | jq -r ".number")
        local url=$(echo ${issue_body} | jq -r '.html_url')
        local title=$(echo ${issue_body} | jq -r '.title')
        local assignees=$(echo ${issue_body} | jq -r '.assignees[]?.login')
        local state=$(echo ${issue_body}| jq -r '.state')
        local labels=$(echo ${issue_body} | jq -r '.labels[]?.name')
        printf "[%s](%s) #%s | %s | %s | %s \n" "${title}" "${url}" "${issue_id}" "$(join_by , ${assignees})" "${state}" "$(join_by , ${labels})"
    done
}

count_total=0
item_count=100
page=1
echo ""
printf "%s | %s | %s | %s \n" "Issue Title" "Assignees" "Issue State" "Labels"
echo "---|---|---|---"
while [ "${item_count}" == "100" ]
do
    gh_get_issues ${MILESTONE_ID} "${LABELS}" "open" ${page}
    item_count=$(echo ${last_issue_list} | jq -r '. | length')
    print_issue_rows 
    page=$((page+1))
    count_total=$((count_total + item_count))
done

if [ -n "$DEBUG" ]; then
echo ""
echo "total items: ${count_total}"
fi
