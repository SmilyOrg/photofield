---
name: security-updates
description: Check and apply security updates across the photofield project (api/Go, ui/npm, docs/npm, e2e/npm). Use when the user asks to update dependencies, patch vulnerabilities, run a security audit, or check for breakage from version upgrades.
version: 1.0.0
platforms: [linux]
---

# Security Updates for Photofield

The project has four dependency trees to audit:

| Area | Type | Package File |
|------|------|-------------|
| **api** | Go | `go.mod` / `go.sum` |
| **ui** | npm | `ui/package.json` |
| **docs** | npm | `docs/package.json` |
| **e2e** | npm | `e2e/package.json` |

## Process

### Step 1: Scan current state

```bash
go version
govulncheck ./... 2>&1
cd ui && npm audit 2>&1
cd ../docs && npm audit 2>&1
cd ../e2e && npm audit 2>&1
```

Record before-counts for the final report.

### Step 2: Apply updates per area

#### API (Go)
```bash
cd <root>
go get -u go
go get -u ./...
go mod tidy
govulncheck ./... 2>&1 | grep -E "(vulnerabilit|affected)"
```

- Go stdlib upgrades require `toolchain goX.Y.Z` in `go.mod` to auto-download.

#### UI / Docs / E2E (npm)
```bash
cd <area>
npm audit 2>&1
npm install <pkg>@latest
```

If transitive dependencies (e.g. `esbuild`, `postcss`, `rollup`) have vulns with no fix in their own tree, add an `overrides` field to `package.json`:

```json
"overrides": { "<package>": "<version>" }
```

Always run `npm audit` again to verify.

### Step 3: Build verification

All areas must build cleanly:

```bash
go build ./... && go test ./... -short 2>&1
cd ui && npx vite build 2>&1
cd ../docs && npx vitepress build 2>&1
cd ../e2e && npx playwright test 2>&1
```

Non-blocking warnings to ignore:
- `govulncheck` reporting vulns in imported packages not called by code
- Chunk size warnings
- Tree-shaking annotation warnings from transitive dependencies

### Step 4: Final audit report

```bash
echo "=== API ==="; go version && govulncheck ./... 2>&1 | grep -E "(vulnerabilit|affected|No vulnerabilities)"
echo "=== UI ==="; cd ui && npm audit 2>&1
echo "=== DOCS ==="; cd ../docs && npm audit 2>&1
echo "=== E2E ==="; cd ../e2e && npm audit 2>&1
```

Report as a table:

```
| Area | Before | After | Status |
|------|--------|-------|--------|
| API (Go) | X | Y | ✅/⚠️ |
| UI (npm) | X | Y | ✅/⚠️ |
| DOCS (npm) | X | Y | ✅/⚠️ |
| E2E (npm) | X | Y | ✅/⚠️ |
```

### Step 5: Commit

```bash
cd <root>
git add -A
git status
git diff --cached --stat
git commit -m "chore: apply security updates"
```

One commit per logical change (e.g. one for Go, one for npm areas).

### Step 6: Document remaining vulnerabilities

For any vulns that could not be fixed, note why:

```markdown
| Area | Vulns | Reason |
|------|-------|--------|
| UI | 1 low | Upstream — latest package version has no patch |
| Docs | 3 | Locked by transitive dependency chain — fix in newer major version |
```

## Important Notes

- Always run `govulncheck` and all builds after changes.
- Work on the `security-update` branch.
- Accept breaking API changes in tests or build tools when they resolve critical vulns.
- Document unfixable vulns and whether a newer major version (alpha/beta) resolves them — but do not upgrade to alpha/beta unless explicitly requested.
