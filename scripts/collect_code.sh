#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output="${1:-${project_root}/catforge-bot-all-code.txt}"
temporary="$(mktemp)"
trap 'rm -f "${temporary}"' EXIT

find "${project_root}" -type f \
  -not -path '*/.git/*' \
  -not -path '*/.agents/*' \
  -not -path '*/.codex/*' \
  -not -path '*/backups/*' \
  -not -path '*/.monitoring-secrets/*' \
  -not -path '*/engine_cpp/build/*' \
  -not -path '*/deploy/certs/*' \
  \( \
    -name '*.go' -o -name '*.mod' -o -name '*.sum' -o -name '*.md' -o \
    -name '*.yml' -o -name '*.yaml' -o -name '*.sql' -o -name '*.proto' -o \
    -name '*.alloy' -o -name '*.conf' -o -name '*.cpp' -o -name '*.hpp' -o -name '*.sh' -o -name '*.json' -o \
    -name 'LICENSE' -o -name 'NOTICE' -o -name 'Dockerfile' -o -name 'CMakeLists.txt' -o -name '.gitignore' -o \
    -name '.dockerignore' -o -name '.env.example' \
  \) -print0 | sort -z | while IFS= read -r -d '' file; do
    relative="${file#"${project_root}/"}"
    printf '\n================================================================================\n'
    printf 'FILE: %s\n' "${relative}"
    printf '================================================================================\n\n'
    sed -n '1,$p' "${file}"
    printf '\n'
  done >"${temporary}"

mv "${temporary}" "${output}"
trap - EXIT
printf 'Collected source into %s\n' "${output}"
