# Release Checklist

This checklist covers the common steps for creating a Photofield release.

## Prerequisites

- [ ] You are on the `main` branch
- [ ] Working directory is clean (no uncommitted changes)
- [ ] All planned features/fixes are merged
- [ ] CI is passing on main branch

## Pre-Release

### Documentation & README

- [ ] Update `docs/` with any new features or configuration changes
- [ ] Update `README.md` if there are significant changes
  - Features list
  - Installation instructions
  - Screenshots/demos if UI changed
- [ ] Update `defaults.yaml` documentation comments if config changed

### Testing & Changelog

- [ ] Document all changes using changie
  - Use `task added` for new features
  - Use `task fixed` for bug fixes
  - Use `task breaking` for breaking changes
  - Use `task deprecated` for deprecations
  - Use `task removed` for removed features
  - Use `task security` for security updates
- [ ] Test locally with embedded build: `task run:embed`
- [ ] Run tests: `task test`
- [ ] Run e2e tests: `task e2e:ci`

## Creating the Release

- [ ] Run `task release`
  - Batches changelog entries into a version file
  - Opens the version file in your editor for review/editing
  - Merges changes into CHANGELOG.md
  - Commits with "Release vX.Y.Z" message
  - Creates git tag

- [ ] Review the release commit: `git show HEAD`
- [ ] Verify the changelog looks good: `task release:changelog`

## Publishing

- [ ] Push the release: `task release:push`

- [ ] **While CI is running**, update external listings:
  - [ ] Update feature list at https://github.com/meichthys/foss_photo_libraries (issue #95)
  - [ ] Announce on Discord: https://discord.gg/qjMxfCMVqM

- [ ] Wait for CI to complete: https://github.com/SmilyOrg/photofield/actions

- [ ] Review the GitHub Release draft
  - Edit release notes if needed
  - Add screenshots/GIFs if applicable (for UI changes or new features)
  - Check attached artifacts are present
  - **Publish the release**

## Post-Release Verification

- [ ] Test Docker image: `docker pull ghcr.io/smilyorg/photofield:latest`
- [ ] Verify Docker tags created at https://github.com/SmilyOrg/photofield/pkgs/container/photofield
- [ ] Monitor for issues
  - GitHub issues
  - Discord feedback

## If Something Goes Wrong

### Before pushing to GitHub

- [ ] Undo the release locally: `task release:undo`
  - Removes the tag and commit
  - Make fixes and try again

### After pushing to GitHub

- [ ] Delete the tag locally: `git tag -d vX.Y.Z`
- [ ] Delete the tag remotely: `git push origin :refs/tags/vX.Y.Z`
- [ ] Delete the release commit locally: `git reset --hard HEAD~1`
- [ ] Force push: `git push -f origin main`
- [ ] Delete GitHub Release draft if created
- [ ] Make fixes and create a new release

## Release Types

### Semantic Versioning (at version 0.x)

- **Patch** (0.X.Y+1): Bug fixes, security patches
  - Use `task fixed` or `task security`
- **Minor** (0.X+1.0): New features, breaking changes (pre-1.0), removals
  - Use `task added`, `task breaking`, `task removed`, or `task deprecated`
- **Major** (1.0.0+): Reserved for 1.0 stable release

Changie automatically determines version bump based on change types.

## Useful Commands

- `task release:version` - Show current/next version
- `task release:changelog` - Preview changelog for latest release
- `task release:title` - Get the release title
- `changie latest` - Get latest version string
- `git describe --tags` - Show current version with git info
- `task check` - Verify dependencies and generated files are up to date
- `task package` - Build all platform binaries locally (for testing)
- `task release:local` - Build and test Docker image locally

## Notes

- Release artifacts include binaries for:
  - Linux (amd64, arm, arm64, 386, loong64, ppc64le, riscv64, s390x)
  - macOS (amd64, arm64)
  - Windows (amd64, arm64, 386)
  - OpenBSD (amd64, arm64)

- Docker images are multi-arch (amd64, arm64)

- CI automatically skips duplicate builds on release commits to main branch

- All releases are created as drafts first, allowing final review before publishing

- Build artifacts are attested for supply chain security
