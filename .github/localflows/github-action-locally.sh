#!/bin/bash

if ! [ -x "$(which act)" ]; then
  curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash
fi

# run act
act --reuse --platform self-hosted=jashbook/golang-lint:1.19-latest --workflows ../workflows/release-notes.yaml
