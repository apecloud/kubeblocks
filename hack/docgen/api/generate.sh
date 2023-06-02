#!/bin/bash
#Copyright (C) 2022-2023 ApeCloud Co., Ltd
#
#This file is part of KubeBlocks project
#
#This program is free software: you can redistribute it and/or modify
#it under the terms of the GNU Affero General Public License as published by
#the Free Software Foundation, either version 3 of the License, or
#(at your option) any later version.
#
#This program is distributed in the hope that it will be useful
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU Affero General Public License for more details.
#
#You should have received a copy of the GNU Affero General Public License
#along with this program.  If not, see <http://www.gnu.org/licenses/>


set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_DIR=${SCRIPT_DIR%/*/*/*}

echo "> Generate API docs from $PROJECT_DIR"
go generate "$PROJECT_DIR/apis/..."


API_DOCS_DIR="$PROJECT_DIR/docs/user_docs/api-reference"
for file in "$API_DOCS_DIR"/*
do
  if [ -f "$file" ] && [[ "$file" == *.md ]]
  then
    filename=$(basename "$file" .md)

    sed -i '' '1i\
---\
title: KubeBlocks '"$filename"' API Reference\
description: KubeBlocks '"$filename"' API Reference\
keywords: ['"$filename"', api]\
sidebar_position: 2\
---\
\
' "$file"
  fi
done
