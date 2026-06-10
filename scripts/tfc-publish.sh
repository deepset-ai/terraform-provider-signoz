#!/usr/bin/env bash
#
# Publish a release of this provider to the deepset HCP Terraform PRIVATE registry.
# Run AFTER the GitHub release for the tag exists (the Release workflow builds the
# GoReleaser artifacts: zips + *_SHA256SUMS + *_SHA256SUMS.sig).
#
# Prereqs:
#   - GPG public key already added to the org private registry (gives you GPG_KEY_ID)
#   - export TFE_TOKEN=<an HCP Terraform user/team token for org 'deepset'>
#   - export GPG_KEY_ID=<the key-id returned when you added the GPG key to TFC>
#   - gh, jq, curl installed
#
# Usage:  ./scripts/tfc-publish.sh 0.0.11-deepset.1
set -euo pipefail

VERSION="${1:?usage: tfc-publish.sh <version, e.g. 0.0.11-deepset.1>}"
ORG=deepset
NS=deepset      # provider namespace == org for a private provider
NAME=signoz
REPO=deepset-ai/terraform-provider-signoz
API=https://app.terraform.io/api
: "${TFE_TOKEN:?set TFE_TOKEN}"
: "${GPG_KEY_ID:?set GPG_KEY_ID (from the GPG key you registered in the TFC registry)}"

jqh=(-H "Authorization: Bearer $TFE_TOKEN" -H "Content-Type: application/vnd.api+json")
work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT

echo "==> downloading GitHub release assets for v$VERSION"
gh release download "v$VERSION" --repo "$REPO" --dir "$work"
shafile="$(ls "$work"/*_SHA256SUMS)"
sigfile="$(ls "$work"/*_SHA256SUMS.sig)"

echo "==> ensuring provider $NS/$NAME exists (private registry)"
curl -fsS "${jqh[@]}" -X POST "$API/v2/organizations/$ORG/registry-providers" \
  -d "{\"data\":{\"type\":\"registry-providers\",\"attributes\":{\"name\":\"$NAME\",\"namespace\":\"$NS\",\"registry-name\":\"private\"}}}" \
  >/dev/null 2>&1 || echo "   (provider already exists — ok)"

echo "==> creating version $VERSION"
ver="$(curl -fsS "${jqh[@]}" -X POST \
  "$API/v2/organizations/$ORG/registry-providers/private/$NS/$NAME/versions" \
  -d "{\"data\":{\"type\":\"registry-provider-versions\",\"attributes\":{\"version\":\"$VERSION\",\"key-id\":\"$GPG_KEY_ID\",\"protocols\":[\"6.0\"]}}}")"
shasums_up="$(echo "$ver" | jq -r '.data.links["shasums-upload"]')"
sig_up="$(echo "$ver" | jq -r '.data.links["shasums-sig-upload"]')"
[ "$shasums_up" != "null" ] || { echo "ERROR creating version:"; echo "$ver" | jq .; exit 1; }

echo "==> uploading SHA256SUMS + .sig"
curl -fsS -T "$shafile" "$shasums_up"
curl -fsS -T "$sigfile" "$sig_up"

echo "==> uploading platform binaries"
for zip in "$work"/*_linux_amd64.zip "$work"/*_darwin_arm64.zip "$work"/*_darwin_amd64.zip; do
  [ -e "$zip" ] || continue
  fn="$(basename "$zip")"
  os="$(echo "$fn" | sed -E 's/.*_([a-z]+)_([a-z0-9]+)\.zip$/\1/')"
  arch="$(echo "$fn" | sed -E 's/.*_([a-z]+)_([a-z0-9]+)\.zip$/\2/')"
  shasum="$(grep " $fn\$" "$shafile" | awk '{print $1}')"
  plat="$(curl -fsS "${jqh[@]}" -X POST \
    "$API/v2/organizations/$ORG/registry-providers/private/$NS/$NAME/versions/$VERSION/platforms" \
    -d "{\"data\":{\"type\":\"registry-provider-version-platforms\",\"attributes\":{\"os\":\"$os\",\"arch\":\"$arch\",\"shasum\":\"$shasum\",\"filename\":\"$fn\"}}}")"
  bin_up="$(echo "$plat" | jq -r '.data.links["provider-binary-upload"]')"
  curl -fsS -T "$zip" "$bin_up"
  echo "   uploaded $os/$arch"
done

echo
echo "DONE. Reference it as:"
echo "  source  = \"app.terraform.io/$NS/$NAME\""
echo "  version = \"$VERSION\""
