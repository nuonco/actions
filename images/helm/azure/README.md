# helm-azure

<!-- facts-start -->

helm and kubelogin for AKS, using the runner-injected kubeconfig.

| | |
| --- | --- |
| image | `nuonco/helm-azure` |
| tags | `4.3.0`, `latest`, `sha-<commit sha>` |
| platforms | `linux/amd64`, `linux/arm64` |
| base image | `mirror.gcr.io/library/alpine:3.21` |
| entrypoint | `/usr/local/bin/helm` |
| signed by | `https://github.com/nuonco/actions/.github/workflows/images.yml@refs/heads/main` |
| dockerfile | [`images/helm/azure/Dockerfile`](./Dockerfile) |
| status | experimental |

Ships:

| tool | version |
| --- | --- |
| `helm` | `4.3.0` |
| `kubelogin` | `0.2.9` |

<!-- facts-end -->

## Usage

Copy this into an image file in your app config. It declares the image as a
`container_image` component and tells Nuon to reject any digest that the images workflow
in this repo did not sign:

<!-- usage-start -->

```toml
# action
name = "helm-azure"
type = "container_image"

[public]
image_url = "nuonco/helm-azure"
tag       = "4.3.0"

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
name    = "helm-releases"
timeout = "5m"
image   = "{{.nuon.components.helm-azure.outputs.image.ref}}"

[[triggers]]
type = "manual"

[[steps]]
name            = "list-releases"
inline_contents = "helm list --all-namespaces"
```

The runner fetches the install's kubeconfig and sets `KUBECONFIG` before the first step,
so the tools in this image talk to the install's cluster with no further setup. Set
`enable_kube_config = false` on actions that do not need cluster access, or
`kubernetes_context = "<name>"` to target a `kubernetes_context` binding other than the
sandbox default.

## Verifying the signature

`cosign` checks the same identity Nuon does:

<!-- verify-start -->

```sh
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/nuonco/actions/.github/workflows/images.yml@' \
  nuonco/helm-azure:4.3.0
```

<!-- verify-end -->

## Notes

- `kubelogin` is on `PATH` for the exec credential plugin in an AKS kubeconfig.
- The tag is the helm version, not a release of this repo, so an action can pin the helm it runs under.
- No `kubectl` here. Use `kubectl-azure` or `terraform-azure` when a step needs both.
- `image` on an action requires the image-backed-actions org feature, which is off by default.
- Steps on an image-backed action use `inline_contents`, not `command`. This example is a single invocation, so it fits
  on one line; a step that chains commands uses a multi-line script with a shebang and `set -eu`.
