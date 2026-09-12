#!/usr/bin/env python3
# /// script
# requires-python = ">=3.9"
# dependencies = []
# ///

"""Discover buildable images under images/ and emit a GitHub Actions build matrix.

An image is any directory containing a non-empty Dockerfile. The image name is the
directory path relative to the root, with separators replaced by dashes, so
`images/kubectl/min/aws/Dockerfile` publishes as `kubectl-min-aws`.

Metadata comes from LABELs on the final stage of the Dockerfile (see images/AGENTS.md).
`org.opencontainers.image.version` pins an image to its own tag; when it is set the
repository release version is not used as a tag for that image.

`.github/scripts/gen_image_docs.py` imports the parsing helpers here to render the
image READMEs, so the Dockerfile stays the single source of truth for both.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shlex
import sys
from pathlib import Path

TITLE_LABEL = "org.opencontainers.image.title"
DESCRIPTION_LABEL = "org.opencontainers.image.description"
VERSION_LABEL = "org.opencontainers.image.version"
EXPERIMENTAL_LABEL = "nuon.co/experimental"

VAR_RE = re.compile(r"\$\{(\w+)(?::[-+]([^}]*))?\}|\$(\w+)")
VERSION_ARG_SUFFIX = "_VERSION"


def logical_lines(text: str) -> list[str]:
    """Yield Dockerfile instructions with comments dropped and continuations joined."""
    lines: list[str] = []
    buf: list[str] = []
    for raw in text.splitlines():
        stripped = raw.strip()
        if stripped.startswith("#"):
            continue
        if not buf and not stripped:
            continue
        continued = stripped.endswith("\\")
        if continued:
            stripped = stripped[:-1].rstrip()
        buf.append(stripped)
        if continued:
            continue
        joined = " ".join(part for part in buf if part).strip()
        buf = []
        if joined:
            lines.append(joined)
    if buf:
        joined = " ".join(part for part in buf if part).strip()
        if joined:
            lines.append(joined)
    return lines


def split_tokens(payload: str) -> list[str]:
    try:
        return shlex.split(payload, posix=True)
    except ValueError:
        return payload.split()


def parse_pairs(payload: str) -> dict[str, str]:
    """Parse a LABEL/ENV payload, supporting `k=v k2=v2` and legacy `k v` forms."""
    tokens = split_tokens(payload)
    if not tokens:
        return {}
    if "=" not in tokens[0]:
        return {tokens[0]: " ".join(tokens[1:])}
    pairs = {}
    for token in tokens:
        if "=" in token:
            key, value = token.split("=", 1)
            pairs[key] = value
    return pairs


def expand(value: str, variables: dict[str, str]) -> str:
    def replace(match: re.Match[str]) -> str:
        name = match.group(1) or match.group(3)
        default = match.group(2)
        if name in variables and variables[name]:
            return variables[name]
        return default or ""

    return VAR_RE.sub(replace, value)


def parse_from(
    payload: str, variables: dict[str, str], stages: dict[str, str]
) -> tuple[str, str]:
    """Return (base image, stage alias) for a FROM payload.

    A FROM that copies a previous stage resolves to that stage's own base image, so a
    multi-stage build still reports the image the final layer actually runs on.
    """
    tokens = [token for token in split_tokens(payload) if not token.startswith("--")]
    if not tokens:
        return "", ""
    base = expand(tokens[0], variables)
    base = stages.get(base, base)
    alias = ""
    for index, token in enumerate(tokens):
        if token.upper() == "AS" and index + 1 < len(tokens):
            alias = tokens[index + 1]
    return base, alias


def parse_exec(payload: str, variables: dict[str, str]) -> list[str]:
    """Parse an ENTRYPOINT/CMD payload in either exec (JSON) or shell form."""
    payload = payload.strip()
    if payload.startswith("["):
        try:
            parsed = json.loads(payload)
        except json.JSONDecodeError:
            parsed = None
        if isinstance(parsed, list):
            return [expand(str(item), variables) for item in parsed]
    return [expand(payload, variables)] if payload else []


def image_meta(dockerfile: Path) -> dict[str, object]:
    """Return the final stage's labels, base image, entrypoint, and pinned versions.

    Labels, base image and entrypoint describe the final stage only; `versions`
    collects every `ARG *_VERSION` default in the file, in declaration order, because
    the tools an image ships are usually pinned in a builder stage.
    """
    globals_: dict[str, str] = {}
    variables: dict[str, str] = {}
    labels: dict[str, str] = {}
    versions: dict[str, str] = {}
    stages: dict[str, str] = {}
    entrypoint: list[str] = []
    base = ""
    seen_from = False

    for line in logical_lines(dockerfile.read_text(encoding="utf-8")):
        parts = line.split(None, 1)
        instruction = parts[0].upper()
        payload = parts[1] if len(parts) > 1 else ""

        if instruction == "FROM":
            seen_from = True
            variables = dict(globals_)
            labels = {}
            entrypoint = []
            base, alias = parse_from(payload, globals_, stages)
            if alias:
                stages[alias] = base
        elif instruction == "ARG":
            for token in split_tokens(payload):
                key, has_default, value = token.partition("=")
                target = variables if seen_from else globals_
                if has_default:
                    target[key] = expand(value, target)
                else:
                    # A bare `ARG FOO` in a stage inherits the global default.
                    target.setdefault(key, "")
                if not seen_from:
                    variables[key] = globals_[key]
                if key.endswith(VERSION_ARG_SUFFIX) and target.get(key):
                    versions.setdefault(key, target[key])
        elif instruction == "ENV":
            for key, value in parse_pairs(payload).items():
                variables[key] = expand(value, variables)
        elif instruction == "LABEL":
            for key, value in parse_pairs(payload).items():
                labels[key] = expand(value, variables)
        elif instruction == "ENTRYPOINT":
            entrypoint = parse_exec(payload, variables)

    return {
        "labels": labels,
        "base": base,
        "entrypoint": entrypoint,
        "versions": versions,
    }


def final_stage_labels(dockerfile: Path) -> dict[str, str]:
    """Return the labels declared on the last stage, with ARG/ENV defaults expanded."""
    return image_meta(dockerfile)["labels"]


def is_buildable(dockerfile: Path) -> bool:
    return any(
        line.split(None, 1)[0].upper() == "FROM"
        for line in logical_lines(dockerfile.read_text(encoding="utf-8"))
    )


def discover(root: Path) -> list[dict[str, object]]:
    images = []
    for dockerfile in sorted(root.rglob("Dockerfile")):
        if not is_buildable(dockerfile):
            print(f"skipping {dockerfile}: no FROM instruction", file=sys.stderr)
            continue
        directory = dockerfile.parent
        name = "-".join(directory.relative_to(root).parts)
        labels = final_stage_labels(dockerfile)
        version = labels.get(VERSION_LABEL, "")
        images.append(
            {
                "name": name,
                "context": directory.as_posix(),
                "dockerfile": dockerfile.as_posix(),
                "title": labels.get(TITLE_LABEL) or name,
                "description": labels.get(DESCRIPTION_LABEL)
                or f"Nuon action image: {name}",
                "version": version,
                "experimental": labels.get(EXPERIMENTAL_LABEL) == "true",
            }
        )
    return images


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default="images", type=Path)
    args = parser.parse_args()

    if not args.root.is_dir():
        print(f"{args.root} is not a directory", file=sys.stderr)
        return 1

    images = discover(args.root)
    matrix = {"include": images}

    print(json.dumps(matrix, indent=2))

    output = os.environ.get("GITHUB_OUTPUT")
    if output:
        with open(output, "a", encoding="utf-8") as handle:
            handle.write(f"matrix={json.dumps(matrix)}\n")
            handle.write(f"count={len(images)}\n")

    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with open(summary, "a", encoding="utf-8") as handle:
            handle.write("### Discovered images\n\n")
            handle.write("| image | version | experimental |\n| --- | --- | --- |\n")
            for image in images:
                handle.write(
                    f"| `{image['name']}` | {image['version'] or 'release'} "
                    f"| {'yes' if image['experimental'] else 'no'} |\n"
                )

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
