#!/bin/bash

if ! [ -x "$(which act)" ]; then
  curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash
fi

# run act
act --reuse --platform self-hosted=jashbook/golang-lint:latest --workflows .github/localflows/cicd-local.yml
