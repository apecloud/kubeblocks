#!/bin/bash
PARAM=$1
TYPE=$2

if [[ $TYPE == 1 ]]; then
  echo "${PARAM/v/}"
else
  echo "${PARAM/-/.}"
fi
