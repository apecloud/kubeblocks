#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# requires `git` and `gh` commands, ref. https://cli.github.com/manual/installation for installation guides.

workdir=$(dirname $0)
. ${workdir}/gh_env
. ${workdir}/functions.bash

TAG=${TAG:-}
GITHUB_REF=${GITHUB_REF:-}
tag_ref_prefix="refs/tags/"

if [ -z "$TAG" ] && [ -n "$GITHUB_REF" ]; then
TAG=${GITHUB_REF#"${tag_ref_prefix}"}
fi

if [ -z "$TAG" ]; then 
    if [ -n "$DEBUG" ]; then echo "EMPTY TAG, NOOP"; fi
    exit 0
fi

if [ -n "$DEBUG" ]; then
set -x
fi
echo "Processing tag ${TAG}"
rel_body=$(gh release \
    --repo ${OWNER}/${REPO} view "${TAG}" \
    --json 'body')

rel_body_text=$(echo ${rel_body} | jq -r '.body')

if [ -n "$DEBUG" ]; then echo $rel_body_text; fi


# set -o noglob
IFS=$'\r\n' rel_items=($rel_body_text)
# set +o noglob

final_rel_notes=""
for val in "${rel_items[@]}";
do
    if [[ $val == "**Full Changelog**"* ]]; then
        final_rel_notes="${final_rel_notes}\r\n\r\n${val}"
        continue
    fi
    # ignore line if contain ${PR_TITLE}
    if [[ $val != "* ${PR_TITLE}"* ]];then
        final_rel_notes="${final_rel_notes}${val}\r\n"
    fi
done

if [ -n "$DEBUG" ]; then echo -e $final_rel_notes; fi

# gh release --repo ${OWNER}/${REPO} edit ${TAG} --notes "$(echo -e ${final_rel_notes})"
