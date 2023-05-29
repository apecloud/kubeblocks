#!/usr/bin/env python3
# -*- coding:utf-8 -*-

# generate kbcli sha256 notes
# 1. open each *.sha256.txt in target direct
# 2. get the contain of the file
# 3. render the template

import os
import sys
from datetime import date
from string import Template



release_note_template_path = "docs/release_notes/template.md"


def main(argv: list[str]) -> None:
    """
    :param: the kbcli version and the current direct
    :return None
    """
    tag_name = argv[1]
    sha256_direct = argv[2]
    release_note_template_path = "docs/release_notes/kbcli_template.md"
    release_note_path = f"docs/release_notes/{tag_name}/kbcli.md"


    template = ""
    try:
        with open(release_note_template_path, "r") as file:
            template = file.read()
    except FileNotFoundError as e:
        print(f"template {release_note_template_path} not found, IGNORED")

    with open(release_note_path,'a') as f_dest: # 没
        f_dest.write(Template(template).safe_substitute(
            kbcli_version = tag_name[1:],
            today = date.today().strftime("%Y-%m-%d"),
        ))
        for file in os.listdir(sha256_direct):
            with open(os.path.join(sha256_direct, file),"r") as f:
                f_dest.write(f.read())
    print("Done")

if __name__ == "__main__":
    main(sys.argv)