# kubectl-gcp

<!-- facts-start -->

kubectl and gke-gcloud-auth-plugin for GKE, using the runner-injected kubeconfig.

| | |
| --- | --- |
| image | `nuonco/kubectl-gcp` |
| tags | `latest`, `sha-<commit sha>` |
| platforms | `linux/amd64`, `linux/arm64` |
| base image | `mirror.gcr.io/library/alpine:3.21` |
| entrypoint | `/usr/local/bin/kubectl` |
| signed by | `https://github.com/nuonco/actions/.github/workflows/images.yml@refs/heads/main` |
| dockerfile | [`images/kubectl/gcp/Dockerfile`](./Dockerfile) |
| status | experimental |

Ships:

| tool | version |
| --- | --- |
| `kubectl` | `1.33.4` |
| `gcloud` | `516.0.0` |

<!-- facts-end -->

## Usage

Copy this into an image file in your app config. It declares the image as a
`container_image` component and tells Nuon to reject any digest that the images workflow
in this repo did not sign:

<!-- usage-start -->

```toml
# action
name = "kubectl-gcp"
type = "container_image"

[public]
image_url = "nuonco/kubectl-gcp"
tag       = "latest"

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
name    = "list-nodes"
timeout = "5m"
image   = "{{.nuon.components.kubectl-gcp.outputs.image.ref}}"

[[triggers]]
type = "manual"

[[steps]]
name            = "list-nodes"
inline_contents = "kubectl get nodes -o wide"
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
  nuonco/kubectl-gcp:latest
```

<!-- verify-end -->

## Notes

- `gke-gcloud-auth-plugin` ships as part of the Google Cloud SDK in `/opt/google-cloud-sdk`, which the image adds to `PATH`, for the exec credential plugin in a GKE kubeconfig.
- `python3` is installed because the Google Cloud SDK needs it at runtime.
- Scoped to GKE work from an action: `kubectl` and the auth plugin on Alpine, nothing else.
- The image entrypoint is `kubectl`, but a step's `inline_contents` replaces it, so call `kubectl` in the contents.
- `image` on an action requires the image-backed-actions org feature, which is off by default.
- Steps on an image-backed action use `inline_contents`, not `command`. This example is a single invocation, so it fits
  on one line; a step that chains commands uses a multi-line script with a shebang and `set -eu`.
