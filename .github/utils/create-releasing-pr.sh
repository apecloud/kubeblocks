#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# requires `git` and `gh` commands, ref. https://cli.github.com/manual/installation for installation guides.

. ./gh_env
. ./functions.bash

echo "Creating ${PR_TITLE}"

result=$(gh api \
    --method POST \
    -H "Accept: application/vnd.github+json" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    /repos/${OWNER}/${REPO}/pulls \
    -f title="${PR_TITLE}" \
    -f head="${HEAD_BRANCH}" \
    -f base="${BASE_BRANCH}" 1> /dev/null)

if [ "$?" != "0" ]; then
    echo "error: ${result}"
    exit 1
else
    echo "PR created"
fi


