#!/bin/bash

# brew install act
uNames=`uname -s`
if [ "$uNames" == "Darwin" ]; then
  if ! [ -x "$(command -v act)" ]; then
    echo "brew install act"
    brew install act
  fi
fi

# run act
act --rm -P self-hosted=jashbook/golang-lint:v1.47.2 -W .github/localflows/cicd-local.yml
