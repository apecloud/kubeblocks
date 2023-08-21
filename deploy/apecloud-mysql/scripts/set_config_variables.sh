#!/bin/bash
function set_config_variables(){
  echo "set config variables [$1]"
  config_file="/conf/$1.cnf"
  config_content=$(sed -n '/\['$1'\]/,/\[/ { /\['$1'\]/d; /\[/q; p; }' $config_file)
  while read line
  do
    if [[ $line =~ ^[a-zA-Z_][a-zA-Z0-9_]*=[a-zA-Z0-9_.]*$ ]]; then
      echo $line
      eval "export $line"
    elif ! [[ -z $line  || $line =~ ^[[:space:]]*# ]]; then 
      echo "bad format: $line"
    fi
  done <<< "$(echo -e "$config_content")"
}
