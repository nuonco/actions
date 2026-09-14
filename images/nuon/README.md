# nuon

<!-- facts-start -->

Nuon CLI, pinned to the ctl-api promotion release.

| | |
| --- | --- |
| image | `nuonco/nuon` |
| title | `nuon-cli` |
| tags | `0.19.1174`, `latest`, `sha-<commit sha>` |
| platforms | `linux/amd64`, `linux/arm64` |
| base image | `mirror.gcr.io/library/debian:bookworm-slim` |
| entrypoint | `/usr/local/bin/nuon` |
| signed by | `https://github.com/nuonco/actions/.github/workflows/images.yml@refs/heads/main` |
| dockerfile | [`images/nuon/Dockerfile`](./Dockerfile) |
| status | experimental |

Ships:

| tool | version |
| --- | --- |
| `nuon` | `0.19.1174` |

<!-- facts-end -->

## Usage

Copy this into an image file in your app config. It declares the image as a
`container_image` component and tells Nuon to reject any digest that the images workflow
in this repo did not sign:

<!-- usage-start -->

```toml
# action
name = "nuon"
type = "container_image"

[public]
image_url = "nuonco/nuon"
tag       = "0.19.1174"

[verification]
require_signature = true

[[verification.authorities]]
type    = "keyless"
issuer  = "https://token.actions.githubusercontent.com"
subject = "https://github.com/nuonco/actions/.github/workflows/images.yml@refs/heads/main"
```

<!-- usage-end -->

Then point an action at the component's digest-pinned `image.ref` output, which pulls
from the install's own registry instead of mirroring a public tag:

```toml
# action
name    = "install-status"
timeout = "5m"
image   = "{{.nuon.components.nuon.outputs.image.ref}}"

[[triggers]]
type = "manual"

[[steps]]
name            = "status"
inline_contents = "nuon --json installs get {{.nuon.install.id}}"

[steps.env_vars]
NUON_API_URL   = "https://api.nuon.co"
NUON_ORG_ID    = "<your org id>"
NUON_API_TOKEN = "<an api token>"
```

The CLI needs `NUON_API_URL`, `NUON_ORG_ID`, and `NUON_API_TOKEN`. The image supplies
none of them — keep the token in a secret rather than in committed config.

## Verifying the signature

`cosign` checks the same identity Nuon does:

<!-- verify-start -->

```sh
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/nuonco/actions/.github/workflows/images.yml@' \
  nuonco/nuon:0.19.1174
```

<!-- verify-end -->

## Notes

- The tag is the CLI version, not a release of this repo. `ARG NUON_VERSION` in the
  Dockerfile drives both the CLI install and `org.opencontainers.image.version`, so
  `nuonco/nuon:<version>` always holds that exact build and nothing resolves at build
  time.
- `.github/workflows/nuon-version.yml` moves the pin when `nuonco/nuon` promotes a
  release, and opens a PR for it.
- The image sets `NUON_NO_TTY=true` and `CI=true`, so the CLI never prompts. Pass
  `--json` when a step parses the output.
- `image` on an action requires the image-backed-actions org feature, which is off by
  default.
- Steps on an image-backed action use `inline_contents`, not `command`. This example is a
  single invocation, so it fits on one line; a step that chains commands uses a
  multi-line script with a shebang and `set -euo pipefail`.
