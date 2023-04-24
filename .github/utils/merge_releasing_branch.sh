#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# requires `git` and `gh` commands, ref. https://cli.github.com/manual/installation for installation guides.

workdir=$(dirname $0)
. ${workdir}/gh_env
. ${workdir}/functions.bash

set -x

git switch ${BASE_BRANCH}
git merge ${HEAD_BRANCH}
git push