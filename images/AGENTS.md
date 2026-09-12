# Action images

Each image lives in `images/<name>/` with a `Dockerfile`, and may nest: `images/kubectl/min/aws/Dockerfile` publishes as
`kubectl-min-aws`. Metadata for humans, the built image, and README generation lives in `LABEL`s on the **final stage**
(not before the first `FROM`, not on builder stages).

Do not use prose comments (`# experimental`) in any way shape or form. Docgen scrapes labels from the Dockerfile; they
also survive on the published image.

## Required labels

```dockerfile
FROM gcr.io/distroless/static-debian12
LABEL org.opencontainers.image.title="dns-checks"
LABEL org.opencontainers.image.description="Check public DNS delegation, SOA, CAA, and DS for a domain."
LABEL nuon.co/experimental="true"
```

| Label                                  | Required          | Notes                                                                                                         |
| -------------------------------------- | ----------------- | ------------------------------------------------------------------------------------------------------------- |
| `org.opencontainers.image.title`       | yes               | Defaults in docs to the directory name if omitted; still set it.                                              |
| `org.opencontainers.image.description` | yes               | One sentence. Docgen fails if missing.                                                                        |
| `nuon.co/experimental`                 | when experimental | Only `"true"` counts. Omit the label for stable images; do not set `"false"` or print `stable` in the README. |
| `org.opencontainers.image.version`     | when pinned       | Publishes this tag instead of the repo release version. May reference an `ARG` from the same stage.           |

`dns-checks` is the reference. Copy its label block when adding an image.

## Publishing

`.github/workflows/images.yml` builds every image with a `Dockerfile` under `images/`, for `linux/amd64` and
`linux/arm64`, and pushes to `docker.io/nuonco/<name>`. The image name is the directory path relative to `images/` with
`/` replaced by `-`; it does not come from the `title` label, so nested variants stay unique.
`.github/scripts/discover_images.py` does the discovery and label parsing — run it locally to see what will build.

Every push publishes the same digest under:

- `sha-<full commit sha>` and `sha-<short sha>` — immutable, always safe to pin an action to
- `latest` — tip of `main`
- the release version (`1.2.3` and `1.2`) on a published GitHub release

An image with `org.opencontainers.image.version` is pinned to something with its own lifecycle (a CLI version, a
vendored tool). Those images publish that version as a tag on every build and are **not** tagged with the repo release
version, so a nuon CLI image stays at e.g. `nuonco/nuon:0.14.2` regardless of which release ships it. To pin to the
value of a build arg:

```dockerfile
ARG NUON_VERSION=0.14.2
FROM mirror.gcr.io/library/alpine:3.21
LABEL org.opencontainers.image.version="${NUON_VERSION}"
```

Images are signed keylessly with cosign using the workflow's GitHub OIDC identity. Verify with:

```sh
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/nuonco/actions/.github/workflows/images.yml@' \
  nuonco/dns-checks:latest
```

Pull requests build every image but never push or sign. Publishing needs the `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`
repo secrets.

Markdown under `images/` is excluded from the `push` and `pull_request` path filters (`'!images/**.md'`), so a
regenerated README never republishes an image. Keep it that way: the docs are generated from the Dockerfiles, and a docs
commit says nothing new about what is in the image.

## Promotion bumps (nuon image)

`images/nuon/Dockerfile` pins the CLI with a global `ARG NUON_VERSION=<version>`. That one value feeds the install
script and `org.opencontainers.image.version`, so it is also the tag `images.yml` publishes. Nothing is resolved at
build time — a build is reproducible and `nuonco/nuon:<version>` always contains that CLI version.

`.github/workflows/nuon-version.yml` moves the pin. It accepts a `repository_dispatch` of type `nuon-promotion` (or a
manual `workflow_dispatch`), rewrites the `ARG`, re-runs `discover_images.py` to confirm the new version reaches the
matrix, and opens `nuon-version/<version>` as a PR. It is idempotent: a dispatch for the version already pinned is a
no-op, and a repeat dispatch for the same version refreshes the existing PR instead of opening a second one.

`nuonco/nuon` triggers it on promotion:

```yaml
- name: Bump the nuon action image
  env:
    GH_TOKEN: ${{ secrets.ACTIONS_DISPATCH_TOKEN }}
    VERSION: ${{ needs.release.outputs.version }}
  run: |
    jq -n --arg version "$VERSION" \
      '{event_type: "nuon-promotion", client_payload: {version: $version}}' \
      | gh api repos/nuonco/actions/dispatches --input -
```

`ACTIONS_DISPATCH_TOKEN` needs `contents: write` on `nuonco/actions` (a fine-grained PAT or GitHub App installation
token); `GITHUB_TOKEN` cannot dispatch across repositories. The payload carries a bare version (`0.19.1174`, a leading
`v` is stripped); anything that is not a semver-shaped string fails the run.

Set the optional `ACTIONS_PR_TOKEN` secret in this repo to have the PR authored by a PAT or App. With the default
`GITHUB_TOKEN`, GitHub does not run `pull_request` workflows on the PR it creates, so `images.yml` will not build the
image until merge.

## README / docgen

`.github/scripts/gen_image_docs.py` renders the docs from the Dockerfiles. Run it after touching a Dockerfile:

```sh
python3 .github/scripts/gen_image_docs.py          # write
python3 .github/scripts/gen_image_docs.py --check  # what CI runs
```

Both scripts are stdlib-only and carry a PEP 723 header, so `uv run .github/scripts/gen_image_docs.py` works too and
pins the interpreter. CI uses `python3`, which needs no setup step on the runner.

Every image has a `README.md` next to its Dockerfile, and `images/README.md` indexes them as a table linking to each
one. Adding an image and running the script scaffolds its README; fill in the prose afterwards.

Generated regions, rewritten in place and never hand-edited:

| Region                                            | Contents                                                                                                             |
| ------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| `<!-- doc-gen-start/end -->` (`images/README.md`) | The index table. Stable images under `## Images`, the rest under `## Experimental`.                                  |
| `<!-- facts-start/end -->`                        | Description, published image, tags, platforms, base image, entrypoint, signing identity, and `*_VERSION` build args. |
| `<!-- usage-start/end -->`                        | The `container_image` component block for the image.                                                                 |
| `<!-- verify-start/end -->`                       | The `cosign verify` command for the documented tag.                                                                  |

Everything outside a region is hand-written and left alone — the action example, the inputs and outputs, and anything
else specific to the image. Keep that prose truthful about what the image can actually do; the generator cannot check
it.

Every action example in these READMEs uses `inline_contents` on its steps; do not document `command` on an image-backed
action until that changes. A step that runs one binary with flags is a single-line string, a step that chains commands
is a multi-line script opening with a shebang:

```toml
[[steps]]
name            = "list-nodes"
inline_contents = "kubectl get nodes -o wide"
```

The component block has to work when pasted into an app config, so the script resolves the tag the same way `images.yml`
does. A pinned image documents its `org.opencontainers.image.version` and the `refs/heads/main` signing identity,
because that tag is republished on every push to main. Everything else documents the release recorded in the index
region:

```markdown
<!-- release: 1.4.0 (refs/tags/v1.4.0) -->
```

Only `--version` advances that marker; every other run reads it back, so `--check`, a local run, and CI all render the
same tags. With no marker — the state before the first release — images document `latest`.

```sh
python3 .github/scripts/gen_image_docs.py --version 1.4.0 --ref refs/tags/v1.4.0
```

The `docs` job in `.github/workflows/images.yml` runs this after the build matrix succeeds, so the docs only move once
the images they describe are actually published. On a release it passes the release tag; on any other publish it
re-renders Dockerfile facts and leaves the documented release alone. Either way it commits to `image-docs/<version>` or
`image-docs/sync` and opens a PR, reusing the open one if there is one.

`.github/workflows/image-docs.yml` runs `--check` on pull requests, including ones that only touch markdown, and fails
on drift. That is the job that catches a hand-edited region.

The published image name comes from the directory, not the `title` label, so the two can differ (`images/dns-checks/`
publishes `nuonco/dns-checks` with `title=dnsutils`). The docs key off the published name and surface `title` as a row
when it differs.
