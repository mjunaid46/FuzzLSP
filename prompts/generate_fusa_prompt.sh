#!/usr/bin/env bash

set -exo pipefail

readonly promptFile=${1-"prompt.txt"}

readonly BasePrompt=$(cat prompt_base.txt)
readonly FusaPrompt=$(cat prompt_fusa.txt)

cat <<EOF > $promptFile
$BasePrompt
$FusaPrompt
EOF
