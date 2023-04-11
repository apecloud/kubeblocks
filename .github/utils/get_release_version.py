#!/usr/bin/env python
# -*- coding:utf-8 -*-

# Get release version from git tag and set the parsed version to
# environment variable REL_VERSION, if found the release note, set
# WITH_RELEASE_NOTES to true.

import os
import sys

gitRef = os.getenv("GITHUB_REF")
tagRefPrefix = "refs/tags/v"

with open(os.getenv("GITHUB_ENV"), "a") as githubEnv:
    if gitRef is None or not gitRef.startswith(tagRefPrefix):
        print("This is not a release tag")
        sys.exit(1)

    releaseVersion = gitRef[len(tagRefPrefix):]
    releaseNotePath = "docs/release_notes/v{}/v{}.md".format(releaseVersion,releaseVersion)

    if gitRef.find("-alpha.") > 0:
        print("Alpha release build from {} ...".format(gitRef))
    elif gitRef.find("-beta.") > 0:
        print("Beta release build from {} ...".format(gitRef))
    elif gitRef.find("-rc.") > 0:
        print("Release Candidate build from {} ...".format(gitRef))
    else:
        print("Checking if {} exists".format(releaseNotePath))
        if os.path.exists(releaseNotePath):
            print("Found {}".format(releaseNotePath))
            githubEnv.write("WITH_RELEASE_NOTES=true\n")
        else:
            print("{} is not found".format(releaseNotePath))
        print("Release build from {} ...".format(gitRef))

    githubEnv.write("REL_VERSION={}\n".format(releaseVersion))
