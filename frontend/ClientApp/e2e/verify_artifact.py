#!/usr/bin/env python3
"""Independently verify a DCS-produced contract PDF is a real, conformant
artifact — PDF/A-3a (veraPDF) and a valid C2PA manifest (c2patool / c2pa-rs) —
using the SAME external validators pdf-core runs in its own suite
(pdf-core/features/steps/dcs_pdf_core_steps.py). The Playwright vertical shells
out to this at every artifact-producing hop so "it exports a PDF" becomes "it
exports the verifiable artifact we promise".

    python3 verify_artifact.py <pdf-path> [--lifecycle proposed|agreed|executed]

Exits non-zero with a diagnostic on any failure. c2patool is downloaded and
cached on first use; veraPDF runs via its official CLI image (docker).
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request

_C2PATOOL_VERSION = os.environ.get("DCS_C2PATOOL_VERSION", "0.26.61")
_C2PATOOL_CACHE_DIR = os.path.join(tempfile.gettempdir(), "dcs-c2patool")
_VERAPDF_IMAGE = os.environ.get("DCS_VERAPDF_IMAGE", "ghcr.io/verapdf/cli:latest")


def _ensure_c2patool() -> str:
    found = shutil.which("c2patool")
    if found:
        return found
    os.makedirs(_C2PATOOL_CACHE_DIR, exist_ok=True)
    archive_name = f"c2patool-v{_C2PATOOL_VERSION}-x86_64-unknown-linux-gnu.tar.gz"
    archive_path = os.path.join(_C2PATOOL_CACHE_DIR, archive_name)
    extract_dir = os.path.join(_C2PATOOL_CACHE_DIR, f"c2patool-v{_C2PATOOL_VERSION}")
    if not os.path.isfile(os.path.join(extract_dir, "c2patool")):
        url = (
            "https://github.com/contentauth/c2pa-rs/releases/download/"
            f"c2patool-v{_C2PATOOL_VERSION}/{archive_name}"
        )
        if not os.path.exists(archive_path):
            with urllib.request.urlopen(url, timeout=120) as resp, open(archive_path, "wb") as fh:
                fh.write(resp.read())
        os.makedirs(extract_dir, exist_ok=True)
        with tarfile.open(archive_path, "r:gz") as archive:
            archive.extractall(extract_dir)
    for root, _dirs, files in os.walk(extract_dir):
        if "c2patool" in files:
            binary = os.path.join(root, "c2patool")
            os.chmod(binary, 0o755)
            return binary
    raise SystemExit("c2patool binary could not be prepared")


def _verapdf(pdf_path: str) -> None:
    artifacts_dir = os.path.dirname(os.path.abspath(pdf_path)) or "."
    name = os.path.basename(pdf_path)
    completed = subprocess.run(
        ["docker", "run", "--rm", "-v", f"{artifacts_dir}:/data", _VERAPDF_IMAGE,
         "-f", "3a", "--format", "text", f"/data/{name}"],
        check=False, capture_output=True, text=True, timeout=300,
    )
    if "PASS" not in completed.stdout:
        raise SystemExit(
            f"veraPDF PDF/A-3a validation FAILED for {name}\n"
            f"stdout:\n{completed.stdout}\nstderr:\n{completed.stderr}"
        )


def _c2patool(pdf_path: str, lifecycle: str | None) -> None:
    binary = _ensure_c2patool()
    completed = subprocess.run(
        [binary, pdf_path, "--detailed"],
        check=False, capture_output=True, text=True, timeout=300,
    )
    if completed.returncode != 0:
        raise SystemExit(
            f"c2patool (c2pa-rs) rejected the C2PA manifest in {os.path.basename(pdf_path)}\n"
            f"stdout:\n{completed.stdout}\nstderr:\n{completed.stderr}"
        )
    if lifecycle:
        # The DCS stamps the SRS lifecycle state as a C2PA assertion; require the
        # expected banner (proposed/agreed/executed) to appear in the validated
        # manifest so negotiation/settle/sign transitions are provable on the
        # artifact itself, not just the DCS's own state column.
        if lifecycle.lower() not in completed.stdout.lower():
            raise SystemExit(
                f"C2PA manifest for {os.path.basename(pdf_path)} does not carry the expected "
                f"lifecycle '{lifecycle}'.\nc2patool output:\n{completed.stdout}"
            )


def main() -> int:
    parser = argparse.ArgumentParser(description="Verify a DCS contract PDF (PDF/A-3a + C2PA).")
    parser.add_argument("pdf")
    parser.add_argument("--lifecycle", default=None,
                        help="expected C2PA lifecycle banner: proposed|agreed|executed")
    args = parser.parse_args()
    if not os.path.isfile(args.pdf):
        raise SystemExit(f"no such PDF: {args.pdf}")
    with open(args.pdf, "rb") as fh:
        if fh.read(5) != b"%PDF-":
            raise SystemExit(f"{args.pdf} is not a PDF")
    _verapdf(args.pdf)
    _c2patool(args.pdf, args.lifecycle)
    print(json.dumps({"pdf": os.path.basename(args.pdf), "pdfa3a": "PASS",
                      "c2pa": "VALID", "lifecycle": args.lifecycle or "n/a"}))
    return 0


if __name__ == "__main__":
    sys.exit(main())
