#!/usr/bin/env python3
# /// script
# requires-python = ">=3.9"
# dependencies = []
# ///

"""Render the image READMEs from the Dockerfiles.

Every image gets its own `README.md` next to its Dockerfile, and `images/README.md`
indexes them. The generated regions carry facts a user can act on without reading the
Dockerfile: the published image name, the tags a build pushes, the signing identity, and
a `container_image` component block that is ready to copy into an app config.

    python3 .github/scripts/gen_image_docs.py                    # write
    python3 .github/scripts/gen_image_docs.py --check             # fail on drift
    python3 .github/scripts/gen_image_docs.py --version 1.4.0     # pin docs to a release

Each region is delimited by `<!-- <name>-start -->` / `<!-- <name>-end -->`; everything
outside them is hand-written and never touched. See images/AGENTS.md.
"""

from __future__ import annotations

import argparse
import difflib
import os
import re
import sys
from pathlib import Path

from discover_images import (
    DESCRIPTION_LABEL,
    EXPERIMENTAL_LABEL,
    TITLE_LABEL,
    VERSION_ARG_SUFFIX,
    VERSION_LABEL,
    image_meta,
    is_buildable,
)

DEFAULT_REPOSITORY = "nuonco/actions"
DEFAULT_NAMESPACE = "nuonco"
DEFAULT_BRANCH = "main"
WORKFLOW = ".github/workflows/images.yml"
OIDC_ISSUER = "https://token.actions.githubusercontent.com"
CONFIG_COMMENT = "# action"
PLATFORMS = ("linux/amd64", "linux/arm64")
INDEX_REGION = "doc-gen"
RELEASE_MARKER = re.compile(r"<!-- release: (?P<version>\S+) \((?P<ref>\S+)\) -->")
VERSION_RE = re.compile(r"\d+\.\d+\.\d+([-.+][0-9A-Za-z.+-]+)?")

SCAFFOLD = """# {name}

<!-- facts-start -->
<!-- facts-end -->

## Usage

<!-- usage-start -->
<!-- usage-end -->

## Verifying the signature

<!-- verify-start -->
<!-- verify-end -->
"""


def region_pattern(name: str) -> re.Pattern[str]:
    return re.compile(
        rf"(?P<start><!-- {re.escape(name)}-start -->)(?P<body>.*?)"
        rf"(?P<end><!-- {re.escape(name)}-end -->)",
        re.DOTALL,
    )


def replace_region(text: str, name: str, body: str) -> str:
    """Rewrite one delimited region, keeping the markers and surrounding prose."""
    pattern = region_pattern(name)
    if not pattern.search(text):
        raise KeyError(name)
    replacement = f"\\g<start>\n\n{body.strip()}\n\n\\g<end>"
    return pattern.sub(lambda match: match.expand(replacement), text, count=1)


def table(rows: list[tuple[str, ...]], header: tuple[str, ...]) -> str:
    # A table with no header labels renders as `| | |`, matching the repo's fact tables.
    titles = " | ".join(header).strip() or " ".join("" for _ in header)
    lines = [
        f"| {titles} |".replace("|  |", "| |"),
        "| " + " | ".join("---" for _ in header) + " |",
    ]
    lines.extend("| " + " | ".join(row) + " |" for row in rows)
    return "\n".join(lines)


def tool_name(arg: str) -> str:
    return arg[: -len(VERSION_ARG_SUFFIX)].lower().replace("_", "-")


class Image:
    """One buildable image, with every fact the docs need already resolved."""

    def __init__(
        self,
        dockerfile: Path,
        root: Path,
        namespace: str,
        repository: str,
        release_version: str,
        release_ref: str,
    ) -> None:
        meta = image_meta(dockerfile)
        labels: dict[str, str] = meta["labels"]  # type: ignore[assignment]

        self.dockerfile = dockerfile
        self.directory = dockerfile.parent
        self.relative_dir = self.directory.relative_to(root)
        self.name = "-".join(self.relative_dir.parts)
        self.component = self.name
        self.title = labels.get(TITLE_LABEL) or self.name
        self.description = (
            labels.get(DESCRIPTION_LABEL) or f"Nuon action image: {self.name}"
        )
        self.experimental = labels.get(EXPERIMENTAL_LABEL) == "true"
        self.pinned = labels.get(VERSION_LABEL, "")
        self.base = str(meta["base"])
        self.entrypoint = list(meta["entrypoint"])  # type: ignore[arg-type]
        self.versions: dict[str, str] = meta["versions"]  # type: ignore[assignment]
        self.image_url = f"{namespace}/{self.name}"
        self.readme = self.directory / "README.md"

        # A pinned image publishes its own version on every build off main, so that tag
        # is signed by the branch identity. A repo release version only ever comes from
        # the release run, whose identity is the tag ref.
        if self.pinned:
            self.tag = self.pinned
            ref = f"refs/heads/{DEFAULT_BRANCH}"
        elif release_version:
            self.tag = release_version
            ref = release_ref or f"refs/tags/v{release_version}"
        else:
            self.tag = "latest"
            ref = f"refs/heads/{DEFAULT_BRANCH}"

        self.subject = f"https://github.com/{repository}/{WORKFLOW}@{ref}"
        self.identity_regexp = f"^https://github.com/{repository}/{WORKFLOW}@"

        tags = [self.tag] if self.tag != "latest" else []
        if not self.pinned and release_version:
            major_minor = ".".join(release_version.split(".")[:2])
            if major_minor and major_minor != release_version:
                tags.append(major_minor)
        tags.extend(["latest", "sha-<commit sha>"])
        self.tags = tags

    def facts(self) -> str:
        rows = [
            ("image", f"`{self.image_url}`"),
        ]
        # The title label is the human name; it is not always the published image name.
        if self.title != self.name:
            rows.append(("title", f"`{self.title}`"))
        rows += [
            ("tags", ", ".join(f"`{tag}`" for tag in self.tags)),
            ("platforms", ", ".join(f"`{platform}`" for platform in PLATFORMS)),
            ("base image", f"`{self.base}`"),
        ]
        if self.entrypoint:
            rows.append(("entrypoint", f"`{' '.join(self.entrypoint)}`"))
        rows.append(("signed by", f"`{self.subject}`"))
        rows.append(("dockerfile", f"[`{self.dockerfile.as_posix()}`](./Dockerfile)"))
        if self.experimental:
            rows.append(("status", "experimental"))

        body = [self.description, "", table(rows, ("", ""))]
        if self.versions:
            body.extend(
                [
                    "",
                    "Ships:",
                    "",
                    table(
                        [
                            (f"`{tool_name(arg)}`", f"`{version}`")
                            for arg, version in self.versions.items()
                        ],
                        ("tool", "version"),
                    ),
                ]
            )
        return "\n".join(body)

    def usage(self) -> str:
        """The component file for this image, ready to copy as-is."""
        return "\n".join(
            [
                "```toml",
                CONFIG_COMMENT,
                f'name = "{self.component}"',
                'type = "container_image"',
                "",
                "[public]",
                f'image_url = "{self.image_url}"',
                f'tag       = "{self.tag}"',
                "",
                "[verification]",
                "require_signature = true",
                "",
                "[[verification.authorities]]",
                'type    = "keyless"',
                f'issuer  = "{OIDC_ISSUER}"',
                f'subject = "{self.subject}"',
                "```",
            ]
        )

    def verify(self) -> str:
        return "\n".join(
            [
                "```sh",
                "cosign verify \\",
                f"  --certificate-oidc-issuer {OIDC_ISSUER} \\",
                f"  --certificate-identity-regexp '{self.identity_regexp}' \\",
                f"  {self.image_url}:{self.tag}",
                "```",
            ]
        )

    def render(self, current: str) -> str:
        text = current or SCAFFOLD.format(name=self.name)
        for name, body in (
            ("facts", self.facts()),
            ("usage", self.usage()),
            ("verify", self.verify()),
        ):
            try:
                text = replace_region(text, name, body)
            except KeyError:
                raise SystemExit(
                    f"{self.readme}: missing <!-- {name}-start --> / "
                    f"<!-- {name}-end --> region"
                )
        return text


def documented_release(index_text: str) -> tuple[str, str]:
    """Return the release the index already documents, so a plain run reproduces it.

    Only the release workflow advances this, by passing --version; every other run —
    local, or the pull request check — reads it back and renders the same tags.
    """
    match = RELEASE_MARKER.search(index_text)
    return (match["version"], match["ref"]) if match else ("", "")


def index(images: list[Image], version: str, ref: str) -> str:
    sections = []
    if version:
        sections.append(f"<!-- release: {version} ({ref}) -->")
    for heading, group in (
        ("## Images", [i for i in images if not i.experimental]),
        ("## Experimental", [i for i in images if i.experimental]),
    ):
        if not group:
            continue
        rows = [
            (
                f"[`{image.name}`](./{image.relative_dir.as_posix()}/README.md)",
                f"`{image.tag}`",
                image.description,
            )
            for image in group
        ]
        sections.append(f"{heading}\n\n{table(rows, ('image', 'tag', 'description'))}")
    if not sections:
        return "No images."
    return "\n\n".join(sections)


def collect(
    root: Path, namespace: str, repository: str, version: str, ref: str
) -> list[Image]:
    images = []
    for dockerfile in sorted(root.rglob("Dockerfile")):
        if not is_buildable(dockerfile):
            print(f"skipping {dockerfile}: no FROM instruction", file=sys.stderr)
            continue
        images.append(Image(dockerfile, root, namespace, repository, version, ref))
    return sorted(images, key=lambda image: image.name)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default="images", type=Path)
    parser.add_argument(
        "--namespace",
        default=os.environ.get("NAMESPACE") or DEFAULT_NAMESPACE,
        help="registry namespace the images publish to",
    )
    parser.add_argument(
        "--repository",
        default=os.environ.get("GITHUB_REPOSITORY") or DEFAULT_REPOSITORY,
        help="repository that builds and signs the images",
    )
    parser.add_argument(
        "--version",
        default="",
        help="release version to document as the tag, e.g. 1.4.0 (default: latest)",
    )
    parser.add_argument(
        "--ref",
        default="",
        help="git ref that published --version (default: refs/tags/v<version>)",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="report drift without writing, for CI",
    )
    args = parser.parse_args()

    if not args.root.is_dir():
        print(f"{args.root} is not a directory", file=sys.stderr)
        return 1

    index_path = args.root / "README.md"
    if not index_path.is_file():
        print(f"{index_path} is missing", file=sys.stderr)
        return 1
    index_current = index_path.read_text(encoding="utf-8")

    version, ref = args.version.lstrip("v"), args.ref
    if not version:
        version, ref = documented_release(index_current)
    if version and not VERSION_RE.fullmatch(version):
        print(f"unusable version '{args.version or version}'", file=sys.stderr)
        return 1
    if version and not ref:
        ref = f"refs/tags/v{version}"

    images = collect(args.root, args.namespace, args.repository, version, ref)
    if not images:
        print(f"no images found under {args.root}", file=sys.stderr)
        return 1

    rendered: dict[Path, tuple[str, str]] = {}
    try:
        rendered[index_path] = (
            index_current,
            replace_region(index_current, INDEX_REGION, index(images, version, ref)),
        )
    except KeyError:
        print(
            f"{index_path}: missing <!-- {INDEX_REGION}-start --> / "
            f"<!-- {INDEX_REGION}-end --> region",
            file=sys.stderr,
        )
        return 1

    for image in images:
        current = (
            image.readme.read_text(encoding="utf-8") if image.readme.is_file() else ""
        )
        rendered[image.readme] = (current, image.render(current))

    changed = [path for path, (current, new) in rendered.items() if current != new]

    if args.check:
        for path in changed:
            current, new = rendered[path]
            label = "new file" if not current else "drift"
            print(
                f"::error file={path}::{label}: run .github/scripts/gen_image_docs.py"
            )
            sys.stdout.writelines(
                difflib.unified_diff(
                    current.splitlines(keepends=True),
                    new.splitlines(keepends=True),
                    fromfile=f"a/{path}",
                    tofile=f"b/{path}",
                )
            )
        if changed:
            return 1
        print(f"{len(rendered)} files up to date")
        return 0

    for path in changed:
        path.write_text(rendered[path][1], encoding="utf-8")
        print(f"wrote {path}")
    if not changed:
        print(f"{len(rendered)} files already up to date")

    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with open(summary, "a", encoding="utf-8") as handle:
            handle.write("### Image docs\n\n")
            handle.write(
                f"Documented tag for release version: `{version or 'latest'}`\n\n"
            )
            handle.write("| file | state |\n| --- | --- |\n")
            for path in rendered:
                state = "updated" if path in changed else "unchanged"
                handle.write(f"| `{path}` | {state} |\n")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
