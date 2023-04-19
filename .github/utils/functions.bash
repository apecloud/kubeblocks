# bash functions

# requires `gh` command, ref. https://cli.github.com/manual/installation for installation guides.

gh_get_issues () {
    # @arg milestone - Milestone ID, if the string none is passed, issues without milestones are returned.
    # @arg state - Can be one of: open, closed, all; Default: open.
    # @arg page - Cardinal value; Default: 1
    # @result $last_issue_list - contains JSON result
    declare milestone="$1"  labels="$2" state="${3:-open}"  page="${4:-1}"

    # GH list issues API ref: https://docs.github.com/en/rest/issues/issues?apiVersion=2022-11-28#list-repository-issues
    local cmd="gh api \
        --method GET \
        --header 'Accept: application/vnd.github+json' \
        --header 'X-GitHub-Api-Version: 2022-11-28' \
        /repos/${OWNER}/${REPO}/issues \
        -F per_page=100 \
        -F page=${page} \
        -f milestone=${milestone} \
        -f labels=${labels} \
        -f state=${state}"
    echo $cmd
    last_issue_list=`gh api \
        --header 'Accept: application/vnd.github+json' \
        --header 'X-GitHub-Api-Version: 2022-11-28' \
        --method GET \
        /repos/${OWNER}/${REPO}/issues \
        -F per_page=100 \
        -F page=${page} \
        -f milestone=${milestone} \
        -f labels=${labels} \
        -f state=${state}`
}


gh_get_issue_body() {
    # @arg issue_id - Github issue ID
    # @result last_issue_body
    # @result last_issue_url
    # @result last_issue_title
    # @result last_issue_state
    # @result last_issue_assignees
    # @result last_issue_assignees_printable
    declare issue_id="$1" 

    local issue_body=$(gh api \
        --method GET \
        --header 'Accept: application/vnd.github+json' \
        --header 'X-GitHub-Api-Version: 2022-11-28' \
        /repos/${OWNER}/${REPO}/issues/${issue_id})
    local url=$(echo ${issue_body} | jq -r '.url')
    local title=$(echo ${issue_body} | jq -r '.title')
    local assignees=$(echo ${issue_body} | jq -r '.assignees[]?.login')
    local assignees_printable=
    for assignee in ${assignees}
    do 
        assignees_printable="${assignees_printable},${assignee}"
    done
    local assignees_printable=${assignees_printable#,}
    local state=$(echo ${issue_body}| jq -r '.state')

    last_issue_body="${issue_body}"
    last_issue_url="${url}"
    last_issue_title="${title}"
    last_issue_state="${state}"
    last_issue_assignees=${assignees}
    last_issue_assignees_printable=${assignees_printable}
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
    echo "req_data=$req_data"
    
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