#!/bin/bash

if ! [ -x "$(which act)" ]; then
  echo "sudo is required to install Github act tool."
  curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash
fi

# run act
act --reuse --platform self-hosted=jashbook/golang-lint:1.20-latest --workflows .github/localflows/cicd-local.yml
