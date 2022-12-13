#!/usr/bin/env bash
# cfn-test-create-inputs.sh
#
# This tool generates json files in the inputs/ for `cfn test`.
#

set -o errexit
set -o nounset
set -o pipefail
set -x

WORDTOREMOVE="template."
function usage {
    echo "usage:$0 <project_name>"
    echo "Creates a new project and an Cluster for testing"
}

if [ "$#" -ne 2 ]; then usage; fi
if [[ "$*" == help ]]; then usage; fi

rm -rf inputs
mkdir inputs


projectName="${1}"
projectId=$(mongocli iam projects list --output json | jq --arg NAME "${projectName}" -r '.results[] | select(.name==$NAME) | .id')
if [ -z "$projectId" ]; then
    projectId=$(mongocli iam projects create "${projectName}" --output=json | jq -r '.id')

    echo -e "Created project \"${projectName}\" with id: ${projectId}\n"
else
    echo -e "FOUND project \"${projectName}\" with id: ${projectId}\n"
fi

echo "Check if a project is created $projectId"

cd "$(dirname "$0")" || exit
for inputFile in inputs_*;
do
  username="testing@mongodb.com"
  if [[ $inputFile == *"_invalid.template"* ]]; then
    echo "Changing username to be invalid"
    username="(*&)(*&*&)(*&(*&"
  fi
  outputFile=${inputFile//$WORDTOREMOVE/};
  jq --arg pubkey "$MCLI_PUBLIC_API_KEY" \
     --arg pvtkey "$MCLI_PRIVATE_API_KEY" \
   --arg ProjectId "$projectId" \
   --arg username "$username" \
   '.ProjectId?|=$ProjectId |.Username?|=$username |.ApiKeys.PublicKey?|=$pubkey | .ApiKeys.PrivateKey?|=$pvtkey' \
   "$inputFile" > "../inputs/$outputFile"
done
cd ..

ls -l inputs

