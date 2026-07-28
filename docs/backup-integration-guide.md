# Backup & Restore Integration Guide (Administrators)

This guide tells an operator what state a DCS instance holds, how to integrate
it with a backup solution, and how to restore and verify an instance. The DCS
ships no backup tooling of its own; it defines the integration points and the
consistency rules the operator's tooling must respect.

Requirements basis: SRS §2.6 (administrator documentation MUST cover
backup/restore), [DCS-NFR-SF-03] (business continuity & disaster recovery,
RTO/RPO), [DCS-NFR-PER-03] (availability & resilience), [DCS-NFR-SEC-14]
(encryption of sensitive data at rest — applies to backup targets),
[DCS-NFR-SEC-13] (secure data disposal — applies to backup retention),
[DCS-NFR-COMP-03] (GDPR).

## 1. Stateful inventory

One DCS instance (one Helm release) holds durable state in exactly four
places. Everything else is stateless or reconstructible.

| # | Component | What it holds | Loss means |
|---|---|---|---|
| 1 | PostgreSQL server (`postgresql` sub-chart, PVC) — databases `dcs`, `hydra`, `statuslist` | `dcs`: contracts, templates, workflow state, signatures, archive entries, audit chain heads and checkpoint metadata, outbox, contract-target registry. `hydra`: OAuth2 clients incl. machine credentials (ADR-27; secret hashes only). `statuslist`: credential and contract revocation lists. | Total loss of contract and revocation state. Machine-credential secrets are hashes — a restore restores logins; a loss means re-issuing every credential. |
| 2 | IPFS/Kubo repository (`ipfs` sub-chart, PVC: pinned blocks + MFS) | Contract and template PDFs (C2PA-provenanced), audit log entries, Merkle checkpoints, archive snapshots, generated audit reports. | **The audit trail content exists only here** (Postgres holds heads/metadata, not the entries). PDFs of received peer contracts are stored verbatim and cannot be regenerated. Not re-derivable. |
| 3 | HSM token directory (`<release>-hsm-token` PVC, SoftHSM2 file store) | The instance's private keys (`dcs-did`, `dcs-vc`, `dcs-oid4vp-jar`, `dcs-c2pa`), plus the generated `did.json` and C2PA certificate material. | Loss of the instance's cryptographic identity — see §5, this is the most consequential loss. In production with a hardware HSM this PVC does not apply; use the device's native backup/HA mechanism. |
| 4 | Kubernetes Secrets of the release | `<release>-identity` (did.json), `<release>-c2pa-x5chain`, HSM PIN secret, TSA trust material. | Recreatable by re-running provisioning **only together with the token directory** — the provisioning job derives them from the token's keys. Back them up alongside #3 or accept re-provisioning as part of restore. |

Explicitly **not** backup targets:

- **NATS** — transport only; the outbox in the `dcs` database is the source of
  truth and unpublished events are re-delivered after restart.
- **ORCE flows** — re-imported from the chart (`deployment/helm/charts/orce/flows/`)
  on deploy. Exception: if a local TSA is used, its certificate must remain
  available for timestamp verification — keep the TSA trust Secret with #4.
- **Fuseki / semantic hub, Federated Catalogue (+ `fc-postgres`, Keycloak)** —
  re-provisioned/re-registered on deploy; per ADR-18 the catalogue is not
  authoritative state of a single DCS instance.
- **pdf-core** — stateless by design (ADR-4, keyless C2PA flow); it holds no
  keys and no artifacts.

## 2. Consistency rules

The `dcs` database stores IPFS CIDs (`pdf_ipfs_cid`, `snapshot_cid`,
checkpoint and audit-entry CIDs); the IPFS repository stores the objects. A
backup pair is consistent when **every CID referenced by the database snapshot
resolves in the IPFS snapshot**.

- **Order: PostgreSQL first, IPFS second.** IPFS is append-oriented (objects
  are added and pinned; nothing rewrites them), so an IPFS snapshot taken
  after the database dump is a superset of what the dump references. Extra
  unreferenced blocks are harmless.
- Archive deletion unpins objects but no garbage collection is scheduled, so
  unpinned blocks linger. Do not rely on that: treat an unpin as "may vanish",
  which the ordering rule above already accommodates.
- Both backups should come from the same backup window. Cross-window pairs
  (old database + much newer IPFS) remain readable but can resurrect
  contract states the database no longer knows — do not mix windows in a
  restore.
- Crash-consistency is acceptable: all multi-step writes go through the
  transactional outbox, and interrupted processing (PDF regeneration, audit
  anchoring, peer sync) retries after restart. Quiescing the backend
  (scale to 0) during backup is optional, not required.

## 3. Integration points

**PostgreSQL** — either:
- logical: `pg_dump` of each of the three databases (`dcs`, `hydra`,
  `statuslist`) via a CronJob or the backup solution's PostgreSQL agent
  against service `<release>-postgresql:5432`; or
- physical: CSI volume snapshot of the postgres PVC. The Deployment uses the
  `Recreate` strategy, so a snapshot during operation is crash-consistent in
  the same way a power loss is — PostgreSQL recovers via WAL.

Logical dumps are recommended for portability (chart upgrades, storage-class
moves); volume snapshots for speed. Doing both at different cadences is a
reasonable default.

**IPFS** — CSI volume snapshot of the IPFS PVC (covers blocks, pins, and the
MFS mirror in one unit). A logical alternative (`ipfs pin ls` + block export)
exists but has no advantage here; snapshot the volume.

**HSM token directory** — CSI volume snapshot of the token PVC. This volume
contains private key material: the backup copy must be encrypted and
access-restricted at least as strictly as the cluster volume itself
([DCS-NFR-SEC-14]). With a hardware HSM in production, skip this entirely and
follow the vendor's key-backup/HA procedure.

**Secrets** — include the release's Secrets in the cluster-level backup
(velero or equivalent), or accept that a restore re-runs the provisioning job
against the restored token directory to regenerate them.

**Prerequisite for all volume snapshots:** the storage class must support CSI
snapshots, and — per [DCS-NFR-SEC-14] — both the live volumes and the snapshot
target must be encrypted. This is an operator prerequisite; the chart cannot
enforce it.

## 4. Retention, encryption, GDPR

- Backup media hold everything the live system holds, so [DCS-NFR-SEC-14]
  applies to them unchanged: encrypted at rest, access-controlled,
  access-logged.
- **Define and document a retention window.** Erasure obligations
  ([DCS-NFR-COMP-03], [DCS-NFR-SEC-13]) are only fully discharged once the
  erased data has also aged out of every backup generation. A short, explicit
  retention window (e.g. 30 days) bounds the delay between an erasure request
  and its complete effect; an unbounded backup archive makes erasure
  impossible. Record the chosen window in the operator's deletion concept.
- Restores can resurrect erased data. After restoring from a backup taken
  before an erasure was executed, the operator must replay erasures performed
  since that backup (the audit trail records them).

## 5. Restore procedure

1. Deploy the chart into the target namespace with the release's values
   (`helm upgrade --install`), then scale the backend and pdf-core to 0.
2. Restore the HSM token PVC (or attach the hardware HSM), then the release
   Secrets — or re-run the provisioning job against the restored token.
3. Restore PostgreSQL (all three databases) and the IPFS PVC from the same
   backup window.
4. Scale the backend up and `kubectl rollout restart` the Deployments.
5. Verify (§6). Replay any erasures executed after the backup window (§4).

Order matters: the backend refuses to start without HSM, database, and IPFS,
and its startup self-check requires the served `did.json` to match the token's
`dcs-did` key — restoring the database but not the token yields an instance
that authenticates as a different identity than its data claims.

## 6. Post-restore verification

Run all four; each covers a different failure mode:

1. **Audit chain**: `GET /pac/audit/checkpoint/head` returns the latest
   checkpoint; an inclusion proof for a recent entry
   (`GET /pac/audit/checkpoint/proof/{entry_cid}`) verifies. Confirms
   Postgres↔IPFS pairing for the audit trail.
2. **Archive integrity**: `GET /archive/audit` — re-fetches archived
   snapshots from IPFS and compares them against the database. Confirms
   pairing for contract archives.
3. **Artifact retrieval**: export a known contract's PDF via the UI or
   `GET /pdf/export/contract/{did}` and validate its provenance (c2patool).
   Confirms PDFs and the C2PA chain survived.
4. **Identity**: `GET /.well-known/did.json` serves the expected DID, and a
   trusted peer instance still accepts a ship (or the synthetic-peer flow
   passes). Confirms the restored keys are the identity the federation knows.

## 7. Identity loss (worst case)

If the HSM token (or hardware HSM key set) is lost with no backup, the
instance's identity cannot be restored — provisioning generates fresh keys:

- `did.json` changes; peers verify against the served document at every
  exchange, so federation continues after they observe the new document, but
  the eIDAS certificate and any trust-list registration must be renewed.
- Already-issued artifacts remain verifiable: C2PA manifests and JAdES
  signatures embed their certificate chains and do not depend on the current
  did.json.
- New signatures and credentials are issued under the new keys; the operator
  should record the rotation in the audit trail and inform federation peers.

Treat HSM key backup (or hardware-HSM HA) as the highest-priority item in
this guide accordingly.

## 8. RPO/RTO guidance ([DCS-NFR-SF-03])

- RPO is set by the backup cadence of #1 and #2 (the identity state #3/#4
  changes only at provisioning/rotation — event-driven backup is sufficient
  there). Contract and audit data change with every workflow action; daily is
  a floor, not a recommendation.
- RTO is dominated by volume restore time plus one rollout; the chart itself
  redeploys in minutes. Measure both once against a copy of production-sized
  data and record the numbers — [DCS-NFR-SF-03] is verified by documentation
  review and testing, so the measured values belong in the operator's
  runbook, not just this guide.
