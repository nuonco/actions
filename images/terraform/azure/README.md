# terraform-azure

<!-- facts-start -->

terraform, helm, kubectl, git, and kubelogin for AKS, using the runner-injected kubeconfig.

| | |
| --- | --- |
| image | `nuonco/terraform-azure` |
| tags | `1.16.2`, `latest`, `sha-<commit sha>` |
| platforms | `linux/amd64`, `linux/arm64` |
| base image | `mirror.gcr.io/library/alpine:3.21` |
| entrypoint | `/usr/local/bin/terraform` |
| signed by | `https://github.com/nuonco/actions/.github/workflows/images.yml@refs/heads/main` |
| dockerfile | [`images/terraform/azure/Dockerfile`](./Dockerfile) |
| status | experimental |

Ships:

| tool | version |
| --- | --- |
| `terraform` | `1.16.2` |
| `helm` | `4.3.0` |
| `kubectl` | `1.33.4` |
| `kubelogin` | `0.2.9` |

<!-- facts-end -->

## Usage

Copy this into an image file in your app config. It declares the image as a
`container_image` component and tells Nuon to reject any digest that the images workflow
in this repo did not sign:

<!-- usage-start -->

```toml
# action
name = "terraform-azure"
type = "container_image"

[public]
image_url = "nuonco/terraform-azure"
tag       = "1.16.2"

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
name    = "terraform-plan"
timeout = "15m"
image   = "{{.nuon.components.terraform-azure.outputs.image.ref}}"

[[triggers]]
type = "manual"

[[steps]]
name            = "plan"
inline_contents = """
#!/usr/bin/env sh
set -eu
git clone --depth 1 "$MODULE_REPO" /tmp/module
cd /tmp/module
terraform init -input=false
terraform plan -input=false -no-color
"""

[steps.env_vars]
MODULE_REPO = "https://github.com/acme/infra.git"
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
  nuonco/terraform-azure:1.16.2
```

<!-- verify-end -->

## Notes

- `kubelogin` is on `PATH` for the exec credential plugin in an AKS kubeconfig.
- `git` is installed so `terraform init` can fetch git module sources, and so a step can clone the module it plans.
- The tag is the terraform version, not a release of this repo, so an action can pin the terraform it runs under.
- `helm` and `kubectl` are included for the helm and kubernetes providers, and for steps that mix terraform with direct cluster calls.
- An action gets no terraform state backend. Configure one in the module, or keep steps to read-only plans.
- `image` on an action requires the image-backed-actions org feature, which is off by default.
- Steps on an image-backed action use `inline_contents`, not `command`. These examples chain commands, so they use a
  multi-line script; a step that only invokes one binary with flags fits on one line.
