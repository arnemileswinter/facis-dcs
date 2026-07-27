# ADR-23: The contract-data graph is generic — any SHACL vocabulary, field leaves, in-document resolution

Status: Accepted (2026-07-26). **Amends [ADR-22](adr-22-contract-field-model.md)**:
the two-level restriction ("business properties are field references only")
is lifted; the two-level shape remains valid as the depth-one special case.

## Context

ADR-22 separated field declarations (`dcs:contractFields`) from domain data
(`dcs:contractData`) but restricted domain-object properties to bare field
references. Real vocabularies — Gaia-X's LegalPerson with its distinct
legal/headquarter Address nodes, or any external SHACL library — produce
object *graphs* with fixed literals, nested typed objects, and only *some*
properties negotiable. The system must ingest arbitrary SHACL from the
Semantic Hub and validate real instance data against it without
vocabulary-specific code.

## Decision

`dcs:contractData` is an arbitrary object graph. Every domain object is a
typed node named by `@id`. A property value is exactly one of:

- a **literal** (bare JSON scalar or typed `{"@value": …, "@type": …}`) —
  fixed data agreed at authoring time;
- a **reference to a declared `dcs:ContractField`** — a negotiable leaf,
  filled during negotiation (`dcs:value` on the field, one home per value);
- a **reference to another domain object** — structure, arbitrary depth.

Every reference must resolve **in-document** (`validateContractDataGraph`).
Embedded blank nodes are rejected: every node is addressable, so clause
prose, ODRL operands, and KPI observations can name any part of the graph
by IRI.

**Validation against arbitrary libraries.** A SHACL library registered in
the hub (kind `shapes`) joins the active validation graph next to the
canonical shapes and the clause catalog (`HubShapeSource.withClauseCatalog`).
The gate validates a **field-materialized copy** of the document
(`materializeContractDataFields`): a reference to a filled field is
dereferenced to its `dcs:value`, so a vanilla library — written against
plain instance data, knowing nothing of the field indirection — constrains
the live document directly. The stored document is never rewritten; there
is no separate export artifact, because the artifact *is* the interchange
object and its `dcterms:conformsTo`/`sh:shapesGraph` anchors name the
governing shapes.

**Unfilled fields.** While a referenced field is unfilled, the property is
absent from the materialized copy: a library's own cardinality constraint
then names exactly what negotiation still has to deliver. Together with the
offer/closedness gate this means a contract that reaches signing satisfies
both the canonical shapes and every registered library.

## Consequences

- Structural validation shrinks to what SHACL cannot express: the graph is
  closed, nodes are typed and addressable, filled leaves match their
  declared `dcs:datatype`. All vocabulary semantics live in hub shapes.
- The renderer, closedness gate, ODRL binding, and KPI binding are
  unchanged — they already operate on `@id` links and never inspect
  vocabulary terms.
- Supported expressiveness is **SHACL Core** as implemented by the runtime
  engine (goRDFlib, ADR-9), proven by the two-engine corpus
  (`docs/semantic-ontology/linkml/tests/`). SHACL-SPARQL constraints are out
  of scope for the contract gate.
- The template editor derives flat fields from hub shapes today; nested
  (`sh:node`) subform authoring and per-leaf negotiability selection are
  follow-up editor work — the document model and validation gate already
  accept the graphs such an editor will produce.
- Peer-side verification (`VerifyAgainstOriginatorHub`) validates against
  the originator hub's served shapes; a hub serves its registered libraries
  merged, so both sides evaluate the same graph.
