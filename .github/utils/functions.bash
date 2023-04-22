# bash functions

DEBUG=${DEBUG:-}
# requires `gh` command, ref. https://cli.github.com/manual/installation for installation guides.

gh_get_issues () {
    # @arg milestone - Milestone ID, if the string none is passed, issues without milestones are returned.
    # @arg labels - A list of comma separated label names, processed as OR query.
    # @arg state - Can be one of: open, closed, all; Default: open.
    # @arg page - Cardinal value; Default: 1
    # @result $last_issue_list - contains JSON result
    declare milestone="$1" labels="$2" state="${3:-open}"  page="${4:-1}"
    local label_filter=""
    IFS=',' read -ra label_items <<< "${labels}"
    for i in "${label_items[@]}"; do
        label_filter="${label_filter} -f labels=${i}"
    done
    _gh_get_issues ${milestone} "${label_filter}" ${state} ${page}
}

gh_get_issues_with_and_labels () {
    # @arg milestone - Milestone ID, if the string none is passed, issues without milestones are returned.
    # @arg labels - A list of comma separated label names, processed as AND query.
    # @arg state - Can be one of: open, closed, all; Default: open.
    # @arg page - Cardinal value; Default: 1
    # @result $last_issue_list - contains JSON result
    declare milestone="$1" labels="$2" state="${3:-open}" page="${4:-1}"
    _gh_get_issues ${milestone} "-f labels=${labels}" ${state} ${page}
}

_gh_get_issues () {
    # @arg milestone - Milestone ID, if the string none is passed, issues without milestones are returned.
    # @arg label_filter - Label fileter query params.
    # @arg state - Can be one of: open, closed, all; Default: open.
    # @arg page - Cardinal value; Default: 1
    # @result $last_issue_list - contains JSON result
    declare milestone="$1" label_filter="$2" state="${3:-open}" page="${4:-1}"

    # GH list issues API ref: https://docs.github.com/en/rest/issues/issues?apiVersion=2022-11-28#list-repository-issues
    local cmd="gh api \
        --method GET \
        --header 'Accept: application/vnd.github+json' \
        --header 'X-GitHub-Api-Version: 2022-11-28' \
        /repos/${OWNER}/${REPO}/issues \
        -F per_page=100 \
        -F page=${page} \
        -f milestone=${milestone} \
        ${label_filter} \
        -f state=${state}"
    if [ -n "$DEBUG" ]; then echo $cmd; fi
    last_issue_list=`eval ${cmd} 2> /dev/null`
}


gh_get_issue_body() {
    # @arg issue_id - Github issue ID
    # @result last_issue_body
    # @result last_issue_url
    # @result last_issue_title
    # @result last_issue_state
    # @result last_issue_assignees - multi-lines items
    declare issue_id="$1" 

    local issue_body=$(gh api \
        --method GET \
        --header 'Accept: application/vnd.github+json' \
        --header 'X-GitHub-Api-Version: 2022-11-28' \
        /repos/${OWNER}/${REPO}/issues/${issue_id})
    local url=$(echo ${issue_body} | jq -r '.url')
    local title=$(echo ${issue_body} | jq -r '.title')
    local assignees=$(echo ${issue_body} | jq -r '.assignees[]?.login')
    local state=$(echo ${issue_body}| jq -r '.state')
    last_issue_body="${issue_body}"
    last_issue_url="${url}"
    last_issue_title="${title}"
    last_issue_state="${state}"
    last_issue_assignees=${assignees}
}

gh_update_issue_milestone() {
    # @arg issue_id - Github issue ID
    # @arg milestone - Milestone ID, if the string none is passed, issues without milestones are returned.
    # @result last_issue_resp
    declare issue_id="$1" milestone_id="${2:-}"

    if [ -z "$milestone_id" ]; then
        milestone_id=${MILESTONE_ID} 
    fi

    local req_data="{\"milestone\":$milestone_id}"
    
    if [ -n "$DEBUG" ]; then echo "req_data=$req_data"; fi

    local gh_token=$(gh auth token)
    local resp=$(curl \
        --location \
        --request PATCH \
        --header 'Accept: application/vnd.github+json' \
        --header 'X-GitHub-Api-Version: 2022-11-28' \
        --header "Authorization: Bearer ${gh_token}" \
        --data "${req_data}" \
        https://api.github.com/repos/${OWNER}/${REPO}/issues/${issue_id})

    last_issue_resp=${resp}
}

function join_by {
  local d=${1-} f=${2-}
  if shift 2; then
    printf %s "$f" "${@/#/$d}"
  fi
}
