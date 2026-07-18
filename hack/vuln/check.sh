#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Rillan AI LLC
# SPDX-License-Identifier: MIT
#
# Vulnerability gate: govulncheck with a reviewed allowlist.
#
# Runs govulncheck and fails if the code is affected by any vulnerability that
# is NOT listed in hack/vuln/allow.txt. This lets the CI vuln job be *blocking*
# (it catches new vulnerabilities) while explicitly waiving known advisories
# that have no fix and are not reachable in our runtime — each documented with
# rationale in the allowlist.
#
# GOVULNCHECK env var overrides the runner (the Taskfile passes the pinned
# `go run …/govulncheck@vX.Y.Z`); defaults to `govulncheck` on PATH.
set -Eeuo pipefail

cd "$(dirname "$0")/../.."
runner="${GOVULNCHECK:-govulncheck}"
allow_file="hack/vuln/allow.txt"

allow="$(grep -vE '^[[:space:]]*(#|$)' "$allow_file" | awk '{print $1}' | sort -u)"

# Symbol-level findings (trace[0].function set) mean the vulnerable code is
# actually called — that's what govulncheck counts as "your code is affected".
json="$($runner -format json ./...)"
affected="$(printf '%s' "$json" \
  | jq -r 'select(.finding.trace[0].function != null) | .finding.osv' \
  | sort -u)"

unwaived="$(comm -23 <(printf '%s\n' "$affected" | grep -v '^$' || true) \
                     <(printf '%s\n' "$allow"))"

if [ -n "$unwaived" ]; then
  echo "::error::govulncheck found unwaived vulnerabilities:" >&2
  printf '  - %s  (https://pkg.go.dev/vuln/%s)\n' "$unwaived" "$unwaived" >&2
  echo "Fix it, or — if it has no fix and is unreachable — add it to ${allow_file} with rationale." >&2
  exit 1
fi

n_affected="$(printf '%s' "$affected" | grep -c . || true)"
echo "govulncheck: no unwaived vulnerabilities (${n_affected} affecting, all allowlisted)."
