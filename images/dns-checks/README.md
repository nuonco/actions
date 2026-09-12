# DNS Checks

This image contains a `golang` binary which can be used for common DNS verification operations in nuon actions. Common
actions include, checking the status of a delegated Route53 Zone's DNS records.

<!-- facts-start -->

Check public DNS delegation, SOA, CAA, and DS for a domain.

|            |                                                                                  |
| ---------- | -------------------------------------------------------------------------------- |
| image      | `nuonco/dns-checks`                                                              |
| tags       | `latest`, `sha-<commit sha>`                                                     |
| platforms  | `linux/amd64`, `linux/arm64`                                                     |
| base image | `gcr.io/distroless/static-debian12`                                              |
| entrypoint | `/dns-checks`                                                                    |
| signed by  | `https://github.com/nuonco/actions/.github/workflows/images.yml@refs/heads/main` |
| dockerfile | [`images/dns-checks/Dockerfile`](./Dockerfile)                                   |
| status     | experimental                                                                     |

<!-- facts-end -->

## Usage

Copy this into an image file in your app config. It declares the image as a `container_image` component and tells Nuon
to reject any digest that the images workflow in this repo did not sign:

<!-- usage-start -->

```toml
# action
name = "dns-checks"
type = "container_image"

[public]
image_url = "nuonco/dns-checks"
tag       = "latest"

[verification]
require_signature = true

[[verification.authorities]]
type    = "keyless"
issuer  = "https://token.actions.githubusercontent.com"
subject = "https://github.com/nuonco/actions/.github/workflows/images.yml@refs/heads/main"
```

<!-- usage-end -->

Then point an action at the component's digest-pinned `image.ref` output, which pulls from the install's own registry
instead of mirroring a public tag:

```toml
# action
name    = "dns-delegation"
timeout = "5m"
image   = "{{.nuon.components.dns-checks.outputs.image.ref}}"

[[triggers]]
type          = "cron"
cron_schedule = "*/15 * * * *"

[[triggers]]
type = "manual"

[[steps]]
name            = "check-delegation"
inline_contents = "/dns-checks -checks delegation -domain {{.nuon.install.sandbox.outputs.public_domain}} -nameservers {{.nuon.install.sandbox.outputs.public_nameservers}}"
```

### Inputs

Pass every input as a flag on the step's `inline_contents`. Each flag also falls back to its environment variable — the
flag name upper-cased with dashes as underscores (`-caa-issuers` → `CAA_ISSUERS`, `-output` →
`NUON_ACTIONS_OUTPUT_FILEPATH`) — so `[steps.env_vars]` still works for a value shared by several steps.

| Flag                  | Description                                                                 | Default                                |
| --------------------- | --------------------------------------------------------------------------- | -------------------------------------- |
| `-domain`             | Public zone to check.                                                       | none — empty reports `SKIPPED`         |
| `-nameservers`        | Comma-separated nameservers the zone should delegate to.                    | none — empty reports `SKIPPED`         |
| `-checks`             | Comma-separated list: `delegation`, `soa`, `caa`, `ds`, in any combination. | `delegation`                           |
| `-strict-nameservers` | If true, the parent NS set must match `-nameservers` exactly.               | `false` — extra parent NS still pass   |
| `-caa-issuers`        | CAA `issue` tags that must be present when `caa` is selected.               | `letsencrypt.org`                      |
| `-attempts`           | Retraces before giving up.                                                  | `3`                                    |
| `-sleep-seconds`      | Seconds between attempts.                                                   | `20`                                   |
| `-output`             | File to append the JSON outputs to.                                         | the runner's outputs file, else stdout |

The image is distroless, so the step invokes the binary directly and cannot chain commands. Select checks with
`-checks` instead of splitting them across actions. Default stays `delegation` so existing steps keep their old `status`
values. When more than one check is selected, `status` is `OK` or `FAILED` and each check reports its own `*_status`.

To run the full public-zone set in one step:

```toml
[[steps]]
name            = "check-public-zone"
inline_contents = "/dns-checks -checks delegation,soa,caa,ds -strict-nameservers true -domain {{.nuon.install.sandbox.outputs.public_domain}} -nameservers {{.nuon.install.sandbox.outputs.public_nameservers}}"
```

| Check        | What it verifies                                                                                           |
| ------------ | ---------------------------------------------------------------------------------------------------------- |
| `delegation` | Parent NS records point at the install zone. Distinguishes never-delegated from delegated-elsewhere.       |
| `soa`        | Each expected nameserver answers SOA for the zone, so Route 53 is actually serving it.                     |
| `caa`        | Authoritative CAA at the apex allows `-caa-issuers` (Let's Encrypt by default). `EMPTY` or `BLOCKED` fail. |
| `ds`         | Parent has no DS records. Leftover DS on an unsigned Route 53 zone fails DNSSEC-validating resolvers.      |

### Outputs

Format: `json`, appended to `NUON_ACTIONS_OUTPUT_FILEPATH`, or written to stdout when that is unset.

| Value                  | Description                                                                                         |
| ---------------------- | --------------------------------------------------------------------------------------------------- |
| `status`               | Single check: that check's status. Several checks: `OK`, `FAILED`, `SKIPPED`, or `DNS_UNREACHABLE`. |
| `checks`               | The checks that ran.                                                                                |
| `domain`               | The domain that was checked.                                                                        |
| `expected_nameservers` | The `-nameservers` input, verbatim.                                                                 |
| `observed_nameservers` | The NS records the trace found in the parent zone.                                                  |
| `delegated`            | `true`, `false`, or `unknown` when `delegation` ran.                                                |
| `delegation_status`    | `DELEGATED`, `NOT_DELEGATED`, `DELEGATED_ELSEWHERE`, or `DNS_UNREACHABLE`.                          |
| `soa_status`           | `OK` or `FAILED`.                                                                                   |
| `soa_responders`       | Expected nameservers that answered SOA.                                                             |
| `caa_status`           | `OK`, `EMPTY`, or `BLOCKED`.                                                                        |
| `caa_records`          | Authoritative CAA `issue` tags.                                                                     |
| `ds_status`            | `NONE` or `PRESENT`.                                                                                |
| `ds_records`           | Parent DS records, if any.                                                                          |
| `diagnosis`            | A sentence naming the next action to take. Several checks concatenate the failing diagnoses.        |
| `updated_at`           | UTC RFC 3339 timestamp of the check.                                                                |

`delegation` traces from the root instead of resolving the domain, so it tells a zone that was never delegated apart
from one delegated to a stale nameserver set. The process always exits `0` — read `status`, not the exit code.

## Verifying the signature

`cosign` checks the same identity Nuon does:

<!-- verify-start -->

```sh
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/nuonco/actions/.github/workflows/images.yml@' \
  nuonco/dns-checks:latest
```

<!-- verify-end -->

## Notes

- Every step on an image-backed action uses `inline_contents`. The image is distroless: `/dns-checks` and CA
  certificates, no shell and no package manager. A step can only invoke the binary, so keep `inline_contents` to the one
  line that calls it — no shebang, no chained commands.
- `-domain` or `-nameservers` arriving empty means the sandbox outputs are not populated yet, so checks that need them
  report `SKIPPED` instead of failing the action. `ds` can still run from `-domain` alone.
- `image` on an action requires the image-backed-actions org feature, which is off by default.
