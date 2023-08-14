#!/bin/bash
set -e;
STOP_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
printf "{\"stopTime\": \"$STOP_TIME\"}"