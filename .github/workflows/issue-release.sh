#!/bin/bash
TOKEN=$1
REPO=$2
PROJECT_ID=$3
COMPONENT_FIELD_ID=$4
BACKENDPORTAL_OPTION_ID=$5
BODY=$6

echo "TOKEN:"$TOKEN
echo "REPO:"$REPO
echo "PROJECT_ID:"$PROJECT_ID
echo "COMPONENT_FIELD_ID:"$COMPONENT_FIELD_ID
echo "BACKENDPORTAL_OPTION_ID:"$BACKENDPORTAL_OPTION_ID
echo "BODY:"$BODY

GITHUB_API="https://api.github.com"
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

    # get pr_body
    pr_num="${pr_url#*$GITHUB/$REPO/pull/}"
    pr_body_url="$GITHUB_API/repos/$REPO/pulls/$pr_num"
    echo "curl -H \"Authorization: token $TOKEN\" -H \"Accept: application/vnd.github.v3.raw\" $pr_body_url"
    pr_body=`curl -H "Authorization: token $TOKEN" -H "Accept: application/vnd.github.v3.raw" $pr_body_url`
    pr_body=`echo $pr_body|jq ".body"`
    echo "pr_body: "$pr_body

    if [[ $pr_body != null ]] && [[ $pr_body =~ "#" ]]; then
      pr_body=`echo $pr_body| tr -cd "#[0-9]"| sed 's/#/ /g'`
      for issue_num in ${pr_body[@]}; do
        # get issue_node_id
        issue_body_url=$GITHUB_API/repos/$REPO/issues/$issue_num
        echo "curl -H \"Authorization: token $TOKEN\" -H \"Accept: application/vnd.github.v3.raw\" $issue_body_url"
        issue_node_id=`curl -H "Authorization: token $TOKEN" -H "Accept: application/vnd.github.v3.raw" $issue_body_url`
        issue_node_id=`echo $issue_node_id|jq ".node_id"`
        echo "issue_node_id: "$issue_node_id

        # get issue project item_id
        item_id="$( gh api graphql -f query='
          mutation($project:ID!, $issueid:ID!) {
            addProjectNextItem(input: {projectId: $project, contentId: $issueid}) {
              projectNextItem {
                id
              }
            }
          }' -f project=$PROJECT_ID -f issueid=$issue_node_id --jq '.data.addProjectNextItem.projectNextItem.id')"
        echo "item_id: "$item_id

        # set issue to block field
        gh api graphql -f query='
          mutation (
            $project: ID!
            $item: ID!
            $component_field: ID!
            $component_value: String!
          ) {
            set_component: updateProjectNextItemField(input: {
              projectId: $project
              itemId: $item
              fieldId: $component_field
              value: $component_value
            }) {
              projectNextItem {
                id
                }
            }
          }' -f project=$PROJECT_ID -f item=$item_id -f component_field=$COMPONENT_FIELD_ID -f component_value=$BACKENDPORTAL_OPTION_ID --silent
          echo "move issue $item_id done"
      done
    fi
  fi
done
