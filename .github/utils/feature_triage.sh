#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REMOTE_URL=$(git config --get remote.origin.url)
OWNER=$(dirname ${REMOTE_URL} | awk -F ":" '{print $2}')
REPO=$(basename -s .git ${REMOTE_URL})
MILESTONE_ID=${MILESTONE_ID:-5}

ISSUE_LIST=$(gh api \
    --header 'Accept: application/vnd.github+json' \
    --method GET \
    /repos/${OWNER}/${REPO}/issues \
    -F per_page=100 \
    -f milestone=${MILESTONE_ID} \
    -f labels=kind/feature \
    -f state=all)

ROWS=$(echo ${ISSUE_LIST}| jq -r '. | sort_by(.state,.number)| .[].number')


printf "%s | %s | %s | %s | %s | %s\n" "Feature Title" "Assignees" "Issue State" "Code PR Merge Status" "Feature Doc. Status" "Extra Notes"
echo "---|---|---|---|---|---"
for ROW in $ROWS
do 
    ISSUE_ID=$(echo $ROW | awk -F "," '{print $1}')
    ISSUE_BODY=$(gh api \
        --header 'Accept: application/vnd.github+json' \
        --method GET \
        /repos/${OWNER}/${REPO}/issues/${ISSUE_ID})
    URL=$(echo $ISSUE_BODY| jq -r '.url')
    TITLE=$(echo $ISSUE_BODY| jq -r '.title')
    ASSIGNEES=$(echo $ISSUE_BODY| jq -r '.assignees[]?.login')
    ASSIGNEES_PRINTABLE=
    for ASSIGNEE in $ASSIGNEES
    do 
        ASSIGNEES_PRINTABLE="${ASSIGNEES_PRINTABLE},${ASSIGNEE}"
    done
    ASSIGNEES_PRINTABLE=${ASSIGNEES_PRINTABLE#,}
    STATE=$(echo $ISSUE_BODY| jq -r '.state')
    PR=$(echo $ISSUE_BODY| jq -r '.pull_request?.url')
    printf "[%s](%s) #%s | %s | %s | | | \n" "$TITLE" $URL $ISSUE_ID "$ASSIGNEES_PRINTABLE" "$STATE"
done