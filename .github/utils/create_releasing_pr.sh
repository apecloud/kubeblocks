#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# requires `git` and `gh` commands, ref. https://cli.github.com/manual/installation for installation guides.

workdir=$(dirname $0)
. ${workdir}/gh_env
. ${workdir}/functions.bash

set -x

git stash
git switch ${HEAD_BRANCH}
git pull
git rebase origin/${BASE_BRANCH}
git pull
git push

echo "Creating ${PR_TITLE}"
gh pr create --head ${HEAD_BRANCH} --base ${BASE_BRANCH} --title "${PR_TITLE}" --body "" --label "releasing-task"