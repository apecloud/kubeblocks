#!/usr/bin/env python3
# -*- coding:utf-8 -*-

# generate release note for milestone
# 1. get the open issue that named 'v** Release Planning'
# 2. get the milestone URL
# 3. fetch the issues and PRs belonging current milestone
# 4. generate entries to render the release note template

import os
import re
import sys
from datetime import date
from string import Template

from github import Github

RELEASE_ISSUE_RANGE = "^v(.*) Release Planning$"
MAJOR_RELEASE_REGEX = "^([0-9]+\.[0-9]+)\.[0-9]+.*$"
MILESTONE_REGEX = "https://github.com/apecloud/kubeblocks/milestone/([0-9]+)"
CHANGE_TYPES : list[str] = ["New Features", "Bug Fixes", "Miscellaneous"]


def get_change_priority(name: str) -> int:
    if name in CHANGE_TYPES:
        return CHANGE_TYPES.index(name)
    return len(CHANGE_TYPES)


def main(argv: list[str]) -> None:
    changes = []
    warnings = []
    change_lines = []
    breaking_change_lines = []
    gh_env = os.getenv("GITHUB_ENV")
    gh = Github(os.getenv("GITHUB_TOKEN"))

    # get milestone issue
    issues = [
        i
        for i in gh.get_repo("apecloud/kubeblocks").get_issues(state="open")
        if re.search(RELEASE_ISSUE_RANGE, i.title)
    ]
    issues = sorted(issues, key=lambda i: i.id)

    if len(issues) == 0:
        print("FATAL: failed to find issue for release.")
        sys.exit(0)

    if len(issues) > 1:
        print(f"WARNING: found more than one issue for release, so first issue created will be picked: {[i.title for i in issues]}")

    issue = issues[0]
    print(f"Found issue: {issue.title}")

    # get release version from issue name
    release_version = re.search(RELEASE_ISSUE_RANGE, issue.title).group(1)
    print(f"Generating release notes for KubeBlocks {release_version}")

    # Set REL_VERSION
    if gh_env:
        with open(gh_env, "a") as f:
            f.write(f"REL_VERSION={release_version}\n")
            f.write(f"REL_BRANCH=release-{re.search(MAJOR_RELEASE_REGEX, release_version).group(1)}\n")

    release_note_path = f"docs/release_notes/v{release_version}.md"

    # get milestone
    repo_milestones = re.findall(MILESTONE_REGEX, issue.body)
    if len(repo_milestones) == 0:
        print("FATAL: failed to find milestone in release issue body")
        sys.exit(0)
    if len(repo_milestones) > 1:
        print(f"WARNING: found more than one milestone in release issue body, first milestone will be picked: {[i for i in repo_milestones]}")

    # find all issues and PRs in milestone
    repo = gh.get_repo(f"apecloud/kubeblocks")
    milestone = repo.get_milestone(int(repo_milestones[0]))
    issue_or_prs = [i for i in repo.get_issues(milestone, state="closed")]
    print(f"Detected {len(issue_or_prs)} issues or pull requests")

    # find all contributors and build changes
    allContributors = set()
    for issue_or_pr in issue_or_prs:
        url = issue_or_pr.html_url
        try:
            # only a PR can be converted to a PR object, otherwise will throw error.
            pr = issue_or_pr.as_pull_request()
        except:
            continue
        if not pr.merged:
            continue
        contributor = "@" + str(pr.user.login)
        # Auto generate a release note
        note = pr.title.strip()
        change_type = "Miscellaneous"
        title = note.split(":")
        if len(title) > 1:
            prefix = title[0].strip().lower()
            if prefix in ("feat", "feature"):
                change_type = "New Features"
            elif prefix in ("fix", "bug"):
                change_type = "Bug Fixes"
            note = title[1].strip()
        changes.append((change_type, pr, note, contributor, url))
        allContributors.add(contributor)

    last_subtitle = ""
    # generate changes for release notes
    for change in sorted(changes, key=lambda c: (get_change_priority(c[0]), c[1].id)):
        subtitle = change[0]
        if last_subtitle != subtitle:
            last_subtitle = subtitle
            change_lines.append("\n### " + subtitle)
        breaking_change = "breaking-change" in [label.name for label in change[1].labels]
        change_url = " ([#" + str(change[1].number) + "](" + change[4] + ")"
        change_author = ", " + change[3] + ")"
        change_lines.append("- " + change[2] + change_url + change_author)
        if breaking_change:
            breaking_change_lines.append("- " + change[2] + change_url + change_author)

    if len(breaking_change_lines) > 0:
        warnings.append(
            "> **Note: This release contains a few [breaking changes](#breaking-changes).**"
        )

    # generate release note from template
    template = ""
    release_note_template_path = "docs/release_notes/template.md"
    try:
        with open(release_note_template_path, "r") as file:
            template = file.read()
    except FileNotFoundError as e:
        print(f"template {release_note_template_path} not found, IGNORED")

    change_text = "\n".join(change_lines)
    breaking_change_text = "None."
    if len(breaking_change_lines) > 0:
        breaking_change_text = "\n".join(breaking_change_lines)
    
    warnings_text = ""
    if len(warnings) > 0:
        warnings_text = "\n".join(warnings)

    with open(release_note_path, "w") as file:
        file.write(
            Template(template).safe_substitute(
                kubeblocks_version=release_version,
                kubeblocks_changes=change_text,
                kubeblocks_breaking_changes=breaking_change_text,
                warnings=warnings_text,
                kubeblocks_contributors=", ".join(
                    sorted(list(allContributors), key=str.casefold)
                ),
                today=date.today().strftime("%Y-%m-%d"),
            )
        )

    print("Done")


if __name__ == "__main__":
    main(sys.argv)
