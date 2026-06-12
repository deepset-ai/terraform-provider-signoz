# Maintaining & releasing the deepset SigNoz provider fork

This is a **temporary fork** of [`SigNoz/terraform-provider-signoz`](https://github.com/SigNoz/terraform-provider-signoz),
carrying fixes that aren't in an upstream release yet. It's published to deepset's
HCP Terraform **private registry** as `app.terraform.io/deepset/signoz` and consumed
by the `all/saas/signoz` stack in `dc-terraform-live`.

See that stack's [README — *Why a forked provider*](https://github.com/deepset-ai/dc-terraform-live/blob/main/all/saas/signoz/README.md#why-a-forked-provider)
for the rationale, the list of patches, and the exit plan. **The goal is to delete
this fork** once the upstream PRs (#95, #82, #79) ship in a SigNoz release.

## Branches

- **`deepset-fix`** — the patched branch we release from. = upstream `main` + PRs
  #95 + #82 + #79 + the local `jsontypes.Normalized` change. Releases are tagged
  here.
- `main` — tracks upstream `main` (for rebasing / opening the upstream PR).

## One-time setup (already done; documented for the next maintainer)

1. **GPG signing key.** The registry requires GPG-signed releases.
   - A 4096-bit RSA key was generated (`deepset SigNoz provider <deployment@deepset.ai>`,
     key id `6B7F99ACEC3E074F`, **no passphrase** so CI can sign non-interactively).
   - The **private** key is stored *only* as the GitHub Actions secret
     `PRIMUS_GPG_PRIVATE_KEY` on this repo. It is **not** shared person-to-person.
   - The **public** key is registered in the TFC private registry
     (`POST /api/registry/private/v2/gpg-keys`, org `deepset`). The returned
     `key-id` is what `tfc-publish.sh` passes as `GPG_KEY_ID`.
2. **Enable Actions on the fork.** Forks have their workflows dormant until you
   click *"I understand my workflows, go ahead and enable them"* once in the
   repo's **Actions** tab. Without this, tag pushes trigger nothing.

## Making a code change

1. Branch off `deepset-fix`, make the change under `signoz/internal/...`.
2. Build and test locally against a real SigNoz using `dev_overrides` (no registry
   needed):
   ```sh
   # package main is at the REPO ROOT (not signoz/) — build from the root,
   # otherwise you get a ~260KB .a archive instead of the ~67MB provider binary.
   go build -o /tmp/tf-signoz-bin/terraform-provider-signoz .
   ```
   ```hcl
   # /tmp/tf-signoz-bin/dev.tfrc
   provider_installation {
     dev_overrides { "app.terraform.io/deepset/signoz" = "/tmp/tf-signoz-bin" }
     direct {}
   }
   ```
   ```sh
   TF_CLI_CONFIG_FILE=/tmp/tf-signoz-bin/dev.tfrc SIGNOZ_ACCESS_TOKEN=… terraform plan
   ```
   A minimal one-resource config is enough to exercise create → update → destroy.
3. If you touched the schema/docs, run `go generate ./...` and commit
   `docs/` (the `Tests` workflow's `generate` job fails otherwise — harmless for
   publishing, but keep it green for the upstream PR).
4. Merge into `deepset-fix`.

## Cutting a release

Releases use the pre-release version scheme **`0.0.11-deepset.N`** (the `-deepset.N`
suffix marks it a pre-release, so consumers must pin it with an exact `version`).
Bump `N` for each new build.

1. **Tag & push** from `deepset-fix` — this triggers the **Release** workflow
   (GoReleaser builds every platform zip + `SHA256SUMS` + GPG `.sig`, and creates
   the GitHub release):
   ```sh
   git checkout deepset-fix
   git tag v0.0.11-deepset.N
   git push origin v0.0.11-deepset.N
   ```
   Wait for the *Release* run to go green and confirm the GitHub release has assets
   (`gh release view v0.0.11-deepset.N --repo deepset-ai/terraform-provider-signoz`).
   > Re-pushing an existing tag won't re-trigger; delete and re-push if needed.
2. **Publish to the TFC private registry** with the helper (downloads the GitHub
   release assets and uploads them):
   ```sh
   export TFE_TOKEN=<HCP Terraform user/team token for org 'deepset'>
   export GPG_KEY_ID=6B7F99ACEC3E074F   # or the id returned when the GPG key was registered
   ./scripts/tfc-publish.sh 0.0.11-deepset.N
   ```
   On success it prints the `source` / `version` to reference.
3. **Point the stack at the new version.** In `dc-terraform-live`, bump
   `version = "0.0.11-deepset.N"` in all three `terraform.tf`
   (`all/saas/signoz/{dev,prod,_modules/signoz-config}`) and open a PR. State is
   provider-agnostic, so no resource churn.

## Gotchas learned the hard way

- **Pre-release pinning:** `0.0.11-deepset.N` is invisible to `terraform init`
  unless pinned with an exact `version = "..."`. A bare `source` with no version
  fails with *"no available releases match the given constraints."*
- **Schema type changes need state migration.** Switching attributes to
  `jsontypes.Normalized` made the type reject empty-string `""` values that the
  old provider had written to state. We migrated state once (`"" → "{}"`); if you
  change attribute types again, plan for the same.
- **TFC registry is API-only** for GPG keys and provider publishing — there's no
  full UI for it. `tfc-publish.sh` wraps the API calls.
