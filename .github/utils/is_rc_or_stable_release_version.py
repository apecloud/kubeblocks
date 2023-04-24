#!/usr/bin/env python3
# -*- coding:utf-8 -*-

# Get release version from git tag and set the parsed version to
# environment variable REL_VERSION, if found the release note, set
# WITH_RELEASE_NOTES to true.

import os
import sys
from typing import TypeAlias


def main(argv: list[str]) -> None:
    git_ref = os.getenv("GITHUB_REF")
    tag_ref_prefix = "refs/tags/v"
    github_env : str = str(os.getenv("GITHUB_ENV"))


    with open(github_env, "a") as github_env_f:
        if git_ref is None or not git_ref.startswith(tag_ref_prefix):
            print("This is not a release tag")
            sys.exit(1)

        release_version = git_ref[len(tag_ref_prefix) :]
        release_note_path = f"docs/release_notes/v{release_version}/v{release_version}.md"

        def set_with_rel_note_to_true() -> None:
            print(f"Checking if {release_note_path} exists")
            if os.path.exists(release_note_path):
                print(f"Found {release_note_path}")
                github_env_f.write("WITH_RELEASE_NOTES=true\n")
            else:
                print("{} is not found".format(release_note_path))
            print(f"Release build from {git_ref} ...")


        if git_ref.find("-alpha.") > 0:
            print(f"Alpha release build from {git_ref} ...")
            print(f"IGNORED")
        elif git_ref.find("-beta.") > 0:
            print(f"Beta release build from {git_ref} ...")
            print(f"IGNORED")
        elif git_ref.find("-rc.") > 0:
            print(f"Release Candidate build from {git_ref} ...")
            set_with_rel_note_to_true()
        else:
            set_with_rel_note_to_true()

        github_env_f.write(f"REL_VERSION={release_version}\n")


if __name__ == "__main__":
    main(sys.argv)
