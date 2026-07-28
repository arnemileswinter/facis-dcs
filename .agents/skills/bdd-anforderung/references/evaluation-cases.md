# Evaluation cases

Use these cases when maintaining or forward-testing the skill. Do not load
them during a normal requirement run.

| Case | Request shape | Expected routing |
|---|---|---|
| Full product change | Add a new cross-DCS signature-verification flow with a new endpoint and persistence. | Trigger skill, `FULL`; require architecture, discovery, BDD, implementation, verification, revision, and documentation. |
| Compact regression | Correct a local mapping regression whose approved AC and existing requirement-tagged BDD scenario remain unchanged. | Trigger skill, `COMPACT`; analyst, implementer, verifier. Escalate if the existing scenario is insufficient. |
| Incomplete request | “Make the audit checks better.” | Trigger skill, analyst returns `STATUS: NEEDS-INPUT`; no write phase. |
| Source conflict | Ticket permits unsigned final contracts while a binding SRS decision requires signatures. | Trigger skill, return `STATUS: NEEDS-INPUT`; do not resolve the conflict autonomously. |
| Missing environment | Clear product requirement, but a mandatory kind-based proof is unreachable, no equivalent evidence exists, and starting it was not requested. | Determine FULL or COMPACT from the requirement, then stop after the read-only preflight with `STATUS: BLOCKED`. Do not start infrastructure or enter a write phase. |
| Negative trigger | Correct a README typo, tune Docker cache settings, or diagnose a CI runner without changing product behavior. | Do not trigger this skill; use the ordinary scoped workflow. |
