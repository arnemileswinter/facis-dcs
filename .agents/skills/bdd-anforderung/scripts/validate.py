#!/usr/bin/env python3
"""Validate the project-scoped BDD skill and custom-agent registry."""

from __future__ import annotations

import re
import tomllib
from pathlib import Path


SKILL_DIR = Path(__file__).resolve().parents[1]
REPO_ROOT = SKILL_DIR.parents[2]
CONFIG_PATH = REPO_ROOT / ".codex" / "config.toml"
EXPECTED_ROLES = {
    "analyst",
    "architekt",
    "dokumentierer",
    "gherkin-autor",
    "implementierer",
    "revisor",
    "verifier",
}


def load_toml(path: Path) -> dict:
    with path.open("rb") as handle:
        return tomllib.load(handle)


def require(text: str, *needles: str) -> None:
    missing = [needle for needle in needles if needle not in text]
    if missing:
        raise AssertionError(f"missing from {SKILL_DIR / 'SKILL.md'}: {missing}")


def main() -> None:
    config = load_toml(CONFIG_PATH)
    agents = config["agents"]
    assert agents["max_concurrent_threads_per_session"] == 4
    assert "max_threads" not in agents
    assert "max_depth" not in agents

    actual_roles = set(agents) - {
        "max_concurrent_threads_per_session",
        "interrupt_message",
    }
    assert actual_roles == EXPECTED_ROLES

    for role in sorted(EXPECTED_ROLES):
        profile_path = REPO_ROOT / ".codex" / agents[role]["config_file"]
        profile = load_toml(profile_path)
        assert profile["name"] == role
        assert profile["description"] == agents[role]["description"]
        assert profile["sandbox_mode"] in {"read-only", "workspace-write"}

    verifier = load_toml(REPO_ROOT / ".codex" / "agents" / "verifier.toml")
    assert verifier["sandbox_mode"] == "read-only"
    assert "/tmp" in verifier["developer_instructions"]

    skill_text = (SKILL_DIR / "SKILL.md").read_text(encoding="utf-8")
    require(
        skill_text,
        '`fork_turns="none"`',
        "## FULL",
        "## COMPACT",
        "DISCOVERY-GATE",
        "DISCOVERY-GATE: NOT-APPLICABLE",
        "RED-NOT-RUN",
        "STATUS: BLOCKED",
        "implementiere nicht auf Vorrat",
        "freigegebenes Sollverhalten",
        "run_bdd_kind_fast_once",
    )

    metadata = (SKILL_DIR / "agents" / "openai.yaml").read_text(encoding="utf-8")
    match = re.search(r'^  short_description: "([^"]+)"$', metadata, re.MULTILINE)
    assert match and 25 <= len(match.group(1)) <= 64
    assert "$bdd-anforderung" in metadata
    assert "FULL- oder COMPACT-Workflow" in metadata

    evaluations = (
        SKILL_DIR / "references" / "evaluation-cases.md"
    ).read_text(encoding="utf-8")
    for case in (
        "Full product change",
        "Compact regression",
        "Incomplete request",
        "Source conflict",
        "Missing environment",
        "Negative trigger",
    ):
        assert case in evaluations

    gitignore = (REPO_ROOT / ".gitignore").read_text(encoding="utf-8")
    assert "\n/AGENTS.md\n" not in f"\n{gitignore}\n"
    assert "\n.codex/\n" not in f"\n{gitignore}\n"
    assert ".agents/skills/bdd-anforderung/SKILL.md" not in gitignore

    print("BDD skill and agent configuration are valid.")


if __name__ == "__main__":
    main()
