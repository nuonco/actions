# Nuon Action Images

This directory hosts `nuon` managed images for use in
[container actions](https://docs.nuon.co/concepts/actions#container-actions).

Each image has its own README with the facts needed to run it: the published image name,
the tags a build pushes, the tools it ships, and a `container_image` component block that
can be copied straight into an app config. Every image is built for `linux/amd64` and
`linux/arm64` and signed keylessly with cosign by
[`.github/workflows/images.yml`](../.github/workflows/images.yml).

The table below and the generated regions of each image README come from the Dockerfiles.
Run `.github/scripts/gen_image_docs.py` after changing a Dockerfile, and see
[AGENTS.md](./AGENTS.md) for the labels it reads.

<!-- doc-gen-start -->

## Experimental

| image | tag | description |
| --- | --- | --- |
| [`dns-checks`](./dns-checks/README.md) | `latest` | Check public DNS delegation, SOA, CAA, and DS for a domain. |
| [`helm-aws`](./helm/aws/README.md) | `4.3.0` | helm and aws-iam-authenticator for EKS, using the runner-injected kubeconfig. |
| [`helm-azure`](./helm/azure/README.md) | `4.3.0` | helm and kubelogin for AKS, using the runner-injected kubeconfig. |
| [`helm-gcp`](./helm/gcp/README.md) | `4.3.0` | helm and gke-gcloud-auth-plugin for GKE, using the runner-injected kubeconfig. |
| [`kubectl-aws`](./kubectl/aws/README.md) | `latest` | kubectl and aws-iam-authenticator for EKS, using the runner-injected kubeconfig. |
| [`kubectl-azure`](./kubectl/azure/README.md) | `latest` | kubectl and kubelogin for AKS, using the runner-injected kubeconfig. |
| [`kubectl-gcp`](./kubectl/gcp/README.md) | `latest` | kubectl and gke-gcloud-auth-plugin for GKE, using the runner-injected kubeconfig. |
| [`nuon`](./nuon/README.md) | `0.19.1174` | Nuon CLI, pinned to the ctl-api promotion release. |
| [`terraform-aws`](./terraform/aws/README.md) | `1.16.2` | terraform, helm, kubectl, git, and aws-iam-authenticator for EKS, using the runner-injected kubeconfig. |
| [`terraform-azure`](./terraform/azure/README.md) | `1.16.2` | terraform, helm, kubectl, git, and kubelogin for AKS, using the runner-injected kubeconfig. |
| [`terraform-gcp`](./terraform/gcp/README.md) | `1.16.2` | terraform, helm, kubectl, git, and gke-gcloud-auth-plugin for GKE, using the runner-injected kubeconfig. |

<!-- doc-gen-end -->
