#!/usr/bin/env bash
set -euo pipefail

if [ $# -ne 1 ]; then
  echo "Usage: $0 <tag>"
  exit 1
fi

CI_COMMIT_TAG="$1"
DIST_DIR="dist"
TEMPLATE_FILE=".woodpecker/release.md"

mkdir -p "${DIST_DIR}"

echo "CI_COMMIT_TAG=${CI_COMMIT_TAG}"
git tag --sort=-version:refname

PREV_TAG="$(git tag --sort=-version:refname | awk 'NR == 2 { print; exit }')"

echo "PREV_TAG=${PREV_TAG}"

if [ -n "${PREV_TAG}" ]; then
  git log --first-parent --pretty='- %s' "${PREV_TAG}..${CI_COMMIT_TAG}" > "${DIST_DIR}/changelog.txt"
else
  git log --first-parent --pretty='- %s' HEAD > "${DIST_DIR}/changelog.txt"
fi

if [ ! -s "${DIST_DIR}/changelog.txt" ]; then
  printf '%s\n' '- No changes detected.' > "${DIST_DIR}/changelog.txt"
fi

awk -v version="${CI_COMMIT_TAG}" '
  FILENAME == ARGV[1] { changelog = changelog $0 "\n"; next }
  {
    gsub(/\{\{VERSION\}\}/, version)
    if ($0 == "{{CHANGELOG}}") {
      printf "%s", changelog
    } else {
      print
    }
  }
' "${DIST_DIR}/changelog.txt" "${TEMPLATE_FILE}" > "${DIST_DIR}/release.md"

echo
echo "--- changelog.txt ---"
sed -n '1,120p' "${DIST_DIR}/changelog.txt"
echo
echo "--- release.md ---"
sed -n '1,160p' "${DIST_DIR}/release.md"
