#!/usr/bin/env bash
set -euo pipefail
NEW="${1:?usage: set-reactor-version.sh <version>}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

mvn -B -ntp versions:set -DnewVersion="${NEW}" -DgenerateBackupPoms=false
mvn -B -ntp versions:update-child-modules -DgenerateBackupPoms=false

TEST_SERVER="${ROOT}/test-server/pom.xml"
if [[ -f "${TEST_SERVER}" ]]; then
  # -i.bak works with both GNU and BSD (macOS) sed.
  sed -i.bak "s|<chat.lib.version>[^<]*</chat.lib.version>|<chat.lib.version>${NEW}</chat.lib.version>|" "${TEST_SERVER}"
  rm -f "${TEST_SERVER}.bak"
fi
