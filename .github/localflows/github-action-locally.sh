#!/bin/bash

if ! [ -x "$(which act)" ]; then
  echo "To install act authorization, you need to enter your computer password"
  curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash
fi

# run act
act --reuse --platform self-hosted=jashbook/golang-lint:1.19-latest --workflows .github/localflows/cicd-local.yml
