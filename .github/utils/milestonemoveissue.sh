#!/bin/bash

TOKEN=$1
REPO=$2
REPO_OWNER=$3
PROJECT_ID=$4
STATUS_FIELD_ID=$5
TODO_OPTION_ID=$6
ISSUELIST=$7

echo "TOKEN:"$TOKEN
echo "REPO:"$REPO
echo "REPO_OWNER:"$REPO_OWNER
echo "PROJECT_ID:"$PROJECT_ID
echo "STATUS_FIELD_ID:"$STATUS_FIELD_ID
echo "TODO_OPTION_ID:"$TODO_OPTION_ID
echo "ISSUELIST:"$ISSUELIST

#REPO_NAME="${REPO/${REPO_OWNER}\//}"
#GITHUB="https://github.com"
#PR_URLS={}
#i=0

for issue in ${ISSUELIST[@]}; do
  #get issueid
  if [[ ! -z "$issue" ]];then
    IssueID="$(
    gh api graphql -f query='
      query ($REPO_OWNER: String!, $REPO: String!,$issue: Int!){
        repository(owner:$REPO_OWNER, name:$REPO) {
          issue(number:$issue) {
              projectItems(first: 1){
              nodes {
               id
                  }
              }
           }
        }
      }' -f REPO_OWNER=$REPO_OWNER  -f REPO=$REPO -F issue=$issue --jq '.data.repository.issue.projectItems.nodes[].id' )"

    echo "Issue_NUMBER:"$issue
    echo "ITEMID:"$IssueID
  #update issuestatus
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
            }' -f project=$PROJECT_ID -f item=$IssueID -f status_field=$STATUS_FIELD_ID -f status_value=$TODO_OPTION_ID  --silent

  fi
done
