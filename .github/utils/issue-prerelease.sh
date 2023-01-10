#!/bin/bash
TOKEN=$1
REPO=$2
REPO_OWNER=$3
PROJECT_ID=$4
STATUS_FIELD_ID=$5
TODO_OPTION_ID=$6
BODY=$7

echo "TOKEN:"$TOKEN
echo "REPO:"$REPO
echo "REPO_OWNER:"$REPO_OWNER
echo "PROJECT_ID:"$PROJECT_ID
echo "STATUS_FIELD_ID:"$STATUS_FIELD_ID
echo "TODO_OPTION_ID:"$TODO_OPTION_ID
echo "BODY:"$BODY

REPO_NAME="${REPO/${REPO_OWNER}\//}"
GITHUB="https://github.com"
PR_URLS={}
i=0
for pr_url in ${BODY[@]}; do
  pr_url_flag=0
  if [[ $pr_url =~ "http" ]]  && [[ $pr_url =~ "pull" ]] ; then
    pr_url=`echo "$pr_url"| tr -dc "$GITHUB/$REPO/pull/[0-9]"`

    # check pr_url used
    for url_i in ${PR_URLS[@]}; do
      if [[ $pr_url = $url_i ]]; then
        pr_url_flag=1
      fi
    done
    if [[ $pr_url_flag -eq 1 ]]; then
      break
    fi
    PR_URLS[$i]=$pr_url
    i=$[$i+1]
    # get PR Issues References
    PR_NUMBER="${pr_url#*$GITHUB/$REPO/pull/}"
    echo "PR_NUMBER:"$PR_NUMBER
    if [[ ! -z "$PR_NUMBER" ]];then
      closingIssuesReferences="$(
      gh api graphql -f query='
      {
        repository(owner: "'$REPO_OWNER'", name: "'$REPO_NAME'") {
          pullRequest(number: '$PR_NUMBER') {
            id
            closingIssuesReferences (first: 20) {
              edges {
                node {
                  id
                }
              }
            }
          }
        }
      }' --jq '.data.repository.pullRequest.closingIssuesReferences.edges'
      )"
      echo "Closing Issues References:"$closingIssuesReferences
      if [[ -z "$closingIssuesReferences" || "$closingIssuesReferences" == "[]" ]];then
        echo "PR $PR_NUMBER has no Issues References"
      else
        closingIssues=$(echo $closingIssuesReferences | jq -c -r ".[]")
        for closingIssue in ${closingIssues[@]}; do
          # get issue_node_id
          issue_node_id=`echo $closingIssue|jq ".node.id"`
          echo "issue_node_id: "$issue_node_id
          # get issue project item_id
          item_id="$( gh api graphql -f query='
            mutation($project:ID!, $issueid:ID!) {
              addProjectV2ItemById(input: {projectId: $project, contentId: $issueid}) {
                item {
                  id
                }
              }
            }' -f project=$PROJECT_ID -f issueid=$issue_node_id --jq '.data.addProjectV2ItemById.item.id')"
          echo "item_id: "$item_id
          # set issue to block field
          gh api graphql -f query='
            mutation (
              $project: ID!
              $item: ID!
              $status_field: ID!
              $status_value: String!
            ) {
              set_status: updateProjectV2ItemFieldValue(input: {
                projectId: $project
                itemId: $item
                fieldId: $status_field
                value: {
                  singleSelectOptionId: $status_value
                  }
              }) {
                projectV2Item {
                  id
                  }
              }
            }' -f project=$PROJECT_ID -f item=$item_id -f status_field=$STATUS_FIELD_ID -f status_value=$TODO_OPTION_ID --silent
            echo "move issue $item_id done"
        done
      fi
    fi
  fi
done
