---
name: release
description: >-
  Follow the Photofield release process: batch changelog entries, create the
  release commit and tag, and push. Use when the user says "release", "cut a
  release", "create a release", or "prepare vX.Y.Z". Always ask before pushing.
---

# Photofield Release Process

Follow these steps to create a new Photofield release.

## 1. Prerequisites

Before starting, verify all of these:

```bash
# Must be on main
git branch --show-current | grep -q '^main$'
```

```bash
# Local main must be up to date with origin/main (fetch first, then fast-forward)
git fetch origin main
git merge --ff-only origin/main || { echo "NOT UP TO DATE (or diverged)"; exit 1; }
```

```bash
# Working directory must be clean
git diff-index --quiet HEAD -- || { echo "NOT CLEAN"; exit 1; }
```

```bash
# CI must be passing on current HEAD.
# The commit-status endpoint can stay "pending" (total_count 0) even after the
# CI run finished — check the workflow runs instead:
SHA=$(git rev-parse HEAD)
curl -s "https://api.github.com/repos/SmilyOrg/photofield/actions/runs?branch=main&per_page=5" | \
  jq -e --arg sha "$SHA" '[.workflow_runs[] | select(.head_sha == $sha)] | length > 0 and all(.conclusion == "success")' \
  >/dev/null && echo "CI PASSING" || echo "CI NOT PASSING"
```

If any check fails, stop and report the issue to the user. Do not proceed.
If CI is not passing, check why: re-query the same `/actions/runs` endpoint
(or look at the Actions page) to see the run statuses. If a run is still
`in_progress`, wait for it (poll every ~20–30 s; it typically takes a few
minutes) instead of aborting. If a run failed, report it.

## 2. Check for Unreleased Changes

```bash
ls -la .changes/unreleased/
```

If the directory contains only `.gitkeep`, do **not** stop immediately. Commits
may have landed without changelog entries. Check what changed since the last
release:

```bash
git log --oneline $(git describe --tags --abbrev=0)..HEAD
```

If changes shipped without entries (fixes, features, security updates, etc.),
draft entries for them (see entry-writing guidance below) and ask the user
before creating them. Only say *"Nothing to release"* if no substantive
changes exist.

List each entry for the user (read the YAML files):

```bash
for f in .changes/unreleased/*.yaml; do [ -e "$f" ] || continue
  kind=$(grep '^kind:' "$f" | sed 's/kind: //')
  body=$(grep '^body:' "$f" | sed 's/body: //')
  echo "- [$kind] $body"
done
```

### Entry writing guidance

- **Write for users, not developers.** No internal jargon (goroutines, child
  processes, SQL, function names). Keep issue refs `(#NNN)` — they are useful.
  - ❌ `djpeg: reap the child process when decoding fails`
  - ✅ `Stopped a helper process from lingering when decoding a broken image failed`
  - ❌ `Close database without racing the in-flight SQLite writer goroutine`
  - ✅ `Fixed a bug at server shutdown that could lose or corrupt recently updated image information`
- **Only log changes whose effect survives at HEAD.** If a fix was reverted
  before release (e.g. split into a follow-up issue), do not log it.
- **Keep it honest, keep it short.** One entry per distinct user-visible
  change; group tightly related fixes (e.g. one search entry covering
  date filters, scoring, and tool docs) rather than fragmenting.

## 3. Determine Next Version

Changie's auto-bump rules (from `.changie.yaml`):

| Kind               | Bump type | Semantic meaning (at 0.x) |
|--------------------|-----------|---------------------------|
| `Breaking Changes` | minor     | `0.X.0 → 0.(X+1).0`       |
| `Added`            | minor     | `0.X.0 → 0.(X+1).0`       |
| `Removed`          | minor     | `0.X.0 → 0.(X+1).0`       |
| `Deprecated`       | minor     | `0.X.0 → 0.(X+1).0`       |
| `Fixed`            | patch     | `0.X.Y → 0.X.(Y+1)`       |
| `Security`         | patch     | `0.X.Y → 0.X.(Y+1)`       |

**Rule:** If *any* entry is a minor-bump kind, bump **minor**. Otherwise bump **patch**.

```bash
CURRENT=$(changie latest | sed 's/^v//')
MAJOR=$(echo "$CURRENT" | cut -d. -f1)
MINOR=$(echo "$CURRENT" | cut -d. -f2)
PATCH=$(echo "$CURRENT" | cut -d. -f3)
```

Check if any entry is a minor-bump kind:

```bash
# Sets BUMP to "MINOR" if any entry is a minor-bump kind, otherwise "PATCH"
BUMP=$(grep -l -E '^kind: (Breaking Changes|Added|Removed|Deprecated)' .changes/unreleased/*.yaml 2>/dev/null | grep -qv '^$' && echo "MINOR" || echo "PATCH")
```

```bash
if [ "$BUMP" = "MINOR" ]; then
  NEXT="v${MAJOR}.$((MINOR+1)).0"
else
  NEXT="v${MAJOR}.${MINOR}.$((PATCH+1))"
fi
```

## 4. Ask the User

Show the user:

- Current version (`$CURRENT`)
- Unreleased entries (list each kind + body)
- Suggested next version (`$NEXT`)
- Suggested release title (see §5)

**Do not proceed until the user confirms.**

## 5. Set Release Title

Replace the default date-based title with a descriptive one:

| Release type | Title convention |
|---|---|
| Patch | `"Security patches"`, `"Bug fixes and dependency updates"`, `"Stability improvements"` |
| Minor | Short phrase summarizing the main feature(s) |

Front-load the most prominent theme, and **name the feature explicitly** —
especially if the previous release centered on it (e.g.
`"MCP bug fixes and security patches"` rather than the generic
`"Security patches and bug fixes"` when the fixes are all in the MCP
search path).

Example edit for `.changes/$NEXT.md`:

```yaml
oldText: "## [$NEXT] - 2026-06-16"
newText: "## [$NEXT] - Security patches"
```

## 6. Batch and Merge

```bash
# Merge unreleased fragments into version file
changie batch "$NEXT"

# Review the generated file
cat .changes/$NEXT.md

# Set a descriptive title (see §5)
# Use edit to replace the title line in .changes/$NEXT.md

# Merge into CHANGELOG.md
changie merge
```

## 7. Commit and Tag

⚠️ The `task release` target has `changie batch` and `changie merge`
**commented out** — it expects the agent to run them first. Do **not** run
`task release` (it will fail on a dirty tree).

```bash
git add CHANGELOG.md .changes/$NEXT.md
git commit -m "Release $NEXT"
git tag -a "$NEXT" -m "Release $NEXT"
```

## 8. Review

```bash
git show HEAD
task release:changelog
```

Verify:
- Changelog looks correct and well-formatted
- Tag message matches
- Commit only contains expected files (CHANGELOG.md + .changes/$NEXT.md)

Show the review output to the user.

## 9. Push (Only With Approval)

**Always ask before pushing.** Show:

```
Release ready to push:
- Commit: <hash> — Release $NEXT
- Tag: $NEXT
- Changelog: <summary>

Push? (yes/no)
```

Once confirmed:

```bash
git push
git push --tags
```

Then report:

> Release pushed ✓
> - Commit: `<hash>` → `main`
> - Tag: `$NEXT`
> - CI running: https://github.com/SmilyOrg/photofield/actions

## 10. Change the Title After Committing (Before Push)

If the user wants a different title after §7, edit the title line in **both**
`CHANGELOG.md` and `.changes/$NEXT.md`, then amend and re-tag (the commit hash
changes, so the tag must be recreated):

```bash
git add CHANGELOG.md .changes/$NEXT.md
git commit --amend --no-edit
git tag -d "$NEXT"
git tag -a "$NEXT" -m "Release $NEXT"
```

## 11. Undo Procedures

### Before push

```bash
git tag -d "$NEXT"
git reset --hard HEAD~1
```

### After push (Only With Explicit Approval)

```bash
git tag -d "$NEXT"
git push origin :refs/tags/$NEXT
git reset --hard HEAD~1
git push --force-with-lease origin main
```

Then fix the issue and restart from §6.

## Quick Reference

| Command | Purpose |
|---------|---------|
| `changie latest` | Get current latest version |
| `ls .changes/unreleased/` | List unreleased entries |
| `changie batch <v>` | Batch entries into version file |
| `changie merge` | Merge into CHANGELOG.md |
| `changie new -k <kind> -b <body>` | Create a changelog entry inline |
| `git add CHANGELOG.md .changes/<v>.md` | Stage for commit |
| `git commit -m "Release <v>"` | Create release commit |
| `git tag -a <v> -m "Release <v>"` | Create annotated tag |
| `git push && git push --tags` | Push release |
| `task release:changelog` | Preview changelog |
| `task release:undo` | Undo last release (only if not pushed) |

## Common Pitfalls

1. **Dirty working directory** — `task release` will fail. Stage and commit
   changie outputs first.
2. **Wrong version bump** — If you have *any* minor-bump entries but only bump
   patch, the version is wrong. Always check all entry kinds.
3. **Missing `.changes/<v>.md`** — This file must be committed alongside
   `CHANGELOG.md`.
4. **Changie `-e` flag** — `changie new -e` opens an editor. Use `-b` to set
   body inline: `changie new -k Fixed -b "Fix description"`.
5. **`git push` bypasses branch rules** — The repo requires PRs for non-release
   commits. The release commit is exempted (remote logs "Bypassed rule
   violations for refs/heads/main"). This is expected.
6. **`changie new` can "succeed" without writing** — Verify each fragment
   file exists (`ls .changes/unreleased/`) and cat it after creating entries;
   don't trust exit codes alone.
7. **Apostrophes break entry YAML** — Changie quotes bodies with single
   quotes; a `'` inside the text produces invalid YAML. Avoid apostrophes in
   `-b` bodies, or validate afterwards:
   `python3 -c "import glob, yaml; [yaml.safe_load(open(f)) for f in glob.glob('.changes/unreleased/*.yaml')]"`
8. **Commit-status endpoint lag** — `/commits/<sha>/status` can report
   `pending`/`total_count 0` while the CI workflow run has already completed.
   Use the `/actions/runs` API (see §1) and wait for `completed` before
   committing.
