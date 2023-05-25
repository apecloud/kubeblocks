#!/bin/bash
TOKEN=$1
REPO=$2
REPO_OWNER=$3
PROJECT_ID=$4
STATUS_FIELD_ID=$5
TODO_OPTION_ID=$6
RELEASE_NOTES=$7

echo "TOKEN:"$TOKEN
echo "REPO:"$REPO
echo "REPO_OWNER:"$REPO_OWNER
echo "PROJECT_ID:"$PROJECT_ID
echo "STATUS_FIELD_ID:"$STATUS_FIELD_ID
echo "TODO_OPTION_ID:"$TODO_OPTION_ID
echo "RELEASE_NOTES:"$RELEASE_NOTES

RELEASE_NOTE_FILE=""
note_tmp=()
# get the latest version of release_note
for release_note in ${RELEASE_NOTES}/*; do
  set_note=0
  release_note_name=`basename $release_note`
  if [[ $release_note_name =~ "template" ]]; then
    continue
  fi
  release_version="${release_note_name/v/}"
  OLD_IFS="$IFS"
  IFS="."
  rv_array=($release_version)
  IFS="$OLD_IFS"
  # check whether it is the maximum version number
  if [[ -z "$RELEASE_NOTE_FILE" ]]; then
    set_note=1
  elif [[ `echo "${rv_array[0]} > ${note_tmp[0]}"|bc` -eq 1 ]];then
    set_note=1
  elif [[ ${rv_array[0]} = ${note_tmp[0]} && `echo "${rv_array[1]} > ${note_tmp[1]}"|bc` -eq 1 ]];then
    set_note=1
  elif [[ ${rv_array[0]} = ${note_tmp[0]} && ${rv_array[1]} = ${note_tmp[1]}
    && `echo "${rv_array[2]} > ${note_tmp[2]}"|bc` -eq 1 ]];then
    set_note=1
  fi
  # If it is the maximum version number, reassign the `RELEASE_NOTE_FILE`
  if [[ $set_note -eq 1 ]]; then
    RELEASE_NOTE_FILE=$RELEASE_NOTES/$release_note_name
    note_tmp=()
    for rv in ${rv_array[@]}; do
      note_tmp[${#note_tmp[@]}]=$rv
    done
  fi
done
echo "RELEASE_NOTE_FILE:"$RELEASE_NOTE_FILE

REPO_NAME="${REPO/${REPO_OWNER}\//}"
GITHUB="https://github.com"
PR_URLS={}
i=0
# Gets the PR linked issue with `RELEASE_NOTE_FILE`
for pr_url in `cat $RELEASE_NOTE_FILE`; do
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
