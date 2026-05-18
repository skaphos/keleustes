#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Skaphos
# SPDX-License-Identifier: MIT
#
# Lint every PrometheusRule under config/ and assert that every alert
# carries a non-empty `runbook` annotation. Per the observability-stack
# plan §7: "Every alert must have a runbook. Alerts without runbooks fail
# the `task lint` check."
#
# Invoked by `task lint:prometheusrules`. Pinned yq version (v4.46.1).
# Bump deliberately, not via @latest.

set -Eeuo pipefail

YQ_VERSION="${YQ_VERSION:-v4.46.1}"
YQ=(go run "github.com/mikefarah/yq/v4@${YQ_VERSION}")

failures=0
files_checked=0

while IFS= read -r -d '' file; do
  # Skip files that don't actually declare a PrometheusRule. yq returns the
  # empty string when the select matches nothing.
  kind="$("${YQ[@]}" eval 'select(.kind == "PrometheusRule") | .kind' "${file}" 2>/dev/null || true)"
  if [[ -z "${kind}" || "${kind}" == "null" ]]; then
    continue
  fi
  files_checked=$((files_checked + 1))

  # Collect alerts missing a runbook annotation. Empty output = pass.
  missing="$("${YQ[@]}" eval '
    .spec.groups[]
    | .name as $g
    | (.rules // []) []
    | select(has("alert"))
    | select(
        .annotations == null
        or (.annotations.runbook | not)
        or (.annotations.runbook == "")
      )
    | $g + "/" + .alert
  ' "${file}" 2>/dev/null || true)"

  if [[ -n "${missing}" ]]; then
    echo "::error file=${file}::PrometheusRule has alerts missing a runbook annotation:"
    while IFS= read -r line; do
      echo "  ${file}: ${line}"
    done <<< "${missing}"
    failures=$((failures + 1))
  fi
done < <(find config/ -type f \( -name '*.yaml' -o -name '*.yml' \) -print0)

if [[ "${failures}" -gt 0 ]]; then
  echo
  echo "${failures} PrometheusRule file(s) have alerts without runbook annotations."
  echo "Add 'annotations.runbook: https://docs.keleustes.skaphos.io/runbooks/<alert-name>'"
  echo "to every alert. See docs/plans/2026-05-observability-stack.md §7."
  exit 1
fi

echo "lint-prometheusrules: ${files_checked} PrometheusRule file(s) checked, all alerts carry a runbook annotation."
