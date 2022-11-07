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

releaseIssueRegex = "^v(.*) Release Planning$"
majorReleaseRegex = "^([0-9]+\.[0-9]+)\.[0-9]+.*$"
milestoneRegex = "https://github.com/apecloud/kubeblocks/milestone/([0-9]+)"

githubToken = os.getenv("GITHUB_TOKEN")

changeTypes = [
    "New Features",
    "Bug Fixes",
    "Miscellaneous"
]


def get_change_priority(name):
    if name in changeTypes:
        return changeTypes.index(name)
    return len(changeTypes)


changes = []
warnings = []
changeLines = []
breakingChangeLines = []

gh = Github(githubToken)

# get milestone issue
issues = [i for i in gh.get_repo("apecloud/kubeblocks").get_issues(state='open') if
          re.search(releaseIssueRegex, i.title)]
issues = sorted(issues, key=lambda i: i.id)

if len(issues) == 0:
    print("FATAL: failed to find issue for release.")
    sys.exit(0)

if len(issues) > 1:
    print("WARNING: found more than one issue for release, so first issue created will be picked: {}".
          format([i.title for i in issues]))

issue = issues[0]
print("Found issue: {}".format(issue.title))

# get release version from issue name
releaseVersion = re.search(releaseIssueRegex, issue.title).group(1)
print("Generating release notes for KubeBlocks {}".format(releaseVersion))

# Set REL_VERSION
if os.getenv("GITHUB_ENV"):
    with open(os.getenv("GITHUB_ENV"), "a") as githubEnv:
        githubEnv.write("REL_VERSION={}\n".format(releaseVersion))
        githubEnv.write("REL_BRANCH=release-{}\n".format(re.search(majorReleaseRegex, releaseVersion).group(1)))

releaseNotePath = "docs/release_notes/v{}.md".format(releaseVersion)

# get milestone
repoMilestones = re.findall(milestoneRegex, issue.body)
if len(repoMilestones) == 0:
    print("FATAL: failed to find milestone in release issue body")
    sys.exit(0)
if len(repoMilestones) > 1:
    print("WARNING: found more than one milestone in release issue body, first milestone will be picked: {}".
          format([i for i in repoMilestones]))

# find all issues and PRs in milestone
repo = gh.get_repo(f"apecloud/kubeblocks")
milestone = repo.get_milestone(int(repoMilestones[0]))
issueOrPRs = [i for i in repo.get_issues(milestone, state="closed")]
print("Detected {} issues or pull requests".format(len(issueOrPRs)))

# find all contributors and build changes
allContributors = set()
for issueOrPR in issueOrPRs:
    url = issueOrPR.html_url
    try:
        # only a PR can be converted to a PR object, otherwise will throw error.
        pr = issueOrPR.as_pull_request()
    except:
        continue
    if not pr.merged:
        continue
    contributor = "@" + str(pr.user.login)
    # Auto generate a release note
    note = pr.title
    changeType = "Miscellaneous"
    title = note.split(":")
    if len(title) > 1:
        if title[0].lower() in ("feat", "feature"):
            changeType = "New Features"
        elif title[0].lower() in ("fix", "bug"):
            changeType = "Bug Fixes"
        note = title[1]
    changes.append((changeType, pr, note, contributor, url))
    allContributors.add(contributor)

lastSubtitle = ""
# generate changes for release notes
for change in sorted(changes, key=lambda c: (get_change_priority(c[0]), c[1].id)):
    subtitle = change[0]
    if lastSubtitle != subtitle:
        lastSubtitle = subtitle
        changeLines.append("\n### " + subtitle)
    breakingChange = 'breaking-change' in [label.name for label in change[1].labels]
    changeUrl = " ([#" + str(change[1].number) + "](" + change[4] + ")"
    changeAuthor = ", " + change[3] + ")"
    changeLines.append("- " + change[2] + changeUrl + changeAuthor)
    if breakingChange:
        breakingChangeLines.append("- " + change[2] + changeUrl + changeAuthor)

if len(breakingChangeLines) > 0:
    warnings.append("> **Note: This release contains a few [breaking changes](#breaking-changes).**")

# generate release note from template
template = ''
releaseNoteTemplatePath = "docs/release_notes/template.md"
with open(releaseNoteTemplatePath, "r") as file:
    template = file.read()

changeText = "\n".join(changeLines)
breakingChangeText = "None."
if len(breakingChangeLines) > 0:
    breakingChangeText = '\n'.join(breakingChangeLines)
warningsText = ''
if len(warnings) > 0:
    warningsText = '\n'.join(warnings)

with open(releaseNotePath, 'w') as file:
    file.write(Template(template).safe_substitute(
        kubeblocks_version=releaseVersion,
        kubeblocks_changes=changeText,
        kubeblocks_breaking_changes=breakingChangeText,
        warnings=warningsText,
        kubeblocks_contributors=', '.join(sorted(list(allContributors), key=str.casefold)),
        today=date.today().strftime("%Y-%m-%d")))

print("Done")
