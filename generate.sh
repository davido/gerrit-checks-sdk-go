#!/usr/bin/env bash
# Regenerate the Gerrit checks-plugin Go SDK from the OpenAPI document.
#
#   openapi-generator (go) -> drop non-shipped scaffolding
#
# This is the plugin twin of gerrit-sdk-go: the SAME pipeline, fed the checks plugin's
# OWN OpenAPI spec instead of gerrit-core's. That spec is a build output of the checks
# plugin (bazel //plugins/checks:checks_openapi_json), emitted parse-only from the
# plugin's ApiModule bindings and enriched with prose mined from its rest-api*.md docs.
#
# No generated-code patching: the one Gerrit-specific concern (the )]}' XSSI guard) is
# handled by the hand-written transport in gerritxssi/, not by editing output. The go
# generator maps the case-colliding query params O (scalar) / o (array) to distinct
# struct fields on its own, so -- unlike the Rust SDK -- there is no query patch. (Core's
# --parameter-name-mappings r=regexFilter is unneeded here: the checks API has no such
# collision.)
#
# Usage: ./generate.sh [path-or-url]   (default: ./rest-api-openapi.json)
set -euo pipefail
cd "$(dirname "$0")"
SPEC="${1:-rest-api-openapi.json}"

# Reuse the spec straight from a running Gerrit: pass a URL and it is fetched into the
# checked-in snapshot before generation. The checks plugin serves its own document at
#   https://<host>/plugins/checks/Documentation/rest-api-openapi.json
if [[ "$SPEC" == http://* || "$SPEC" == https://* ]]; then
  echo "0/2 fetch spec from $SPEC"
  curl -fsSL "$SPEC" -o rest-api-openapi.json
  SPEC=rest-api-openapi.json
fi

VERSION=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["info"]["version"])' "$SPEC")

echo "1/2 generate go client (checks $VERSION)"
rm -rf checksclient
# enumClassPrefix=true prefixes enum constants with their type name; without it the go
# generator emits bare names (NONE, ALL, FAILED, RUNNING, ...) that collide at package
# scope across CheckState / CheckerStatus / NotifyHandling / BlockingCondition.
npx --yes @openapitools/openapi-generator-cli@2.41.0 generate \
  -g go -i "$SPEC" -o checksclient \
  --additional-properties=packageName=checksclient,withGoMod=false,isGoSubmodule=true,enumClassPrefix=true \
  >/dev/null

echo "2/2 drop non-shipped generator scaffolding"
# test/docs stubs carry placeholder GIT_USER_ID/GIT_REPO_ID imports; the rest is noise.
(cd checksclient && rm -rf test docs api .travis.yml git_push.sh .openapi-generator-ignore README.md .gitignore)

echo "done: checksclient/ regenerated from $SPEC"
