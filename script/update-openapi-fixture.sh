#!/usr/bin/env bash
#
# Refresh the committed OpenAPI fixture used by the spec-validating test client
# (internal/commands/openapi_validation_test.go).
#
# The fixture is a snapshot of the backend's generated OpenAPI document. Tests
# validate every request the SDK sends against it, so it must be refreshed —
# and the diff reviewed/committed — whenever the backend API changes.
#
# Usage:
#   script/update-openapi-fixture.sh                 # local dev backend
#   DHQ_DOCS_URL=https://host/docs.json script/update-openapi-fixture.sh
#
# Requires a running backend (local: `bin/dev` in the deployhq repo). Never run
# in CI — tests read only the committed fixture, keeping them hermetic.

set -euo pipefail

DOCS_URL="${DHQ_DOCS_URL:-https://api.deploy.localhost/docs.json}"
FIXTURE="$(cd "$(dirname "$0")/.." && pwd)/internal/commands/testdata/openapi.json"

echo "Fetching ${DOCS_URL} ..."
tmp="$(mktemp)"
# -k: local dev uses a self-signed certificate. First hit can be slow (cold boot
# + OAS generation), hence the generous timeout.
curl -skf --max-time 180 "${DOCS_URL}" -o "${tmp}"

# Sanity check: must be an OpenAPI document, not an error page.
if ! head -c 100 "${tmp}" | grep -q '"openapi"'; then
  echo "error: response does not look like an OpenAPI document" >&2
  head -c 300 "${tmp}" >&2
  rm -f "${tmp}"
  exit 1
fi

mv "${tmp}" "${FIXTURE}"
echo "Wrote $(wc -c <"${FIXTURE}" | tr -d ' ') bytes to ${FIXTURE}"
echo "Review the diff and commit the fixture together with any SDK changes."
