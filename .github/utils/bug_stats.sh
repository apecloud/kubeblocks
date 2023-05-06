#!/bin/bash

set -o errexit
set -o nounset

# requires `git` and `gh` commands, ref. https://cli.github.com/manual/installation for installation guides.

workdir=$(dirname $0)
. ${workdir}/gh_env
. ${workdir}/functions.bash

bug_report_md_file=${1}

crit_cnt=$(cat ${bug_report_md_file} | grep crit | wc -l)
major_cnt=$(cat ${bug_report_md_file} | grep major | wc -l)
minor_cnt=$(cat ${bug_report_md_file} | grep minor | wc -l)
total_cnt=$(cat ${bug_report_md_file} | wc -l)
total_cnt=$((total_cnt-2))

printf "bug stats\ntotal open: %s\ncritial: %s\nmajor: %s\nminor: %s\n" ${total_cnt} ${crit_cnt} ${major_cnt} ${minor_cnt}