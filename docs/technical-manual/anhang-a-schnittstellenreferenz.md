# Anhang A — Schnittstellenreferenz

Dieser Anhang listet die Schnittstellen einer DCS-Instanz auf drei Ebenen:
die HTTP-API-Gruppen des Backends, die Ereignisse auf dem internen
Event-Bus und die öffentlichen well-known-Endpunkte, über die andere
Instanzen und externe Prüfer die Instanz auflösen.

Die Referenz beschreibt Gruppen und Zweck, nicht einzelne Parameter. Die
vollständige, maschinenlesbare API-Beschreibung liefert jede laufende
Instanz selbst: eine Swagger-Oberfläche und eine
OpenAPI-3-Spezifikation.

Alle fachlichen APIs sind — sofern nicht anders vermerkt — durch das
OIDC-Bearer-Token geschützt; die Scopes im Access Token entsprechen den
Rollen-Claims des Nutzers. Maschinelle Aufrufer (Maschinen-Identitäten
und Contract Target Systems) authentifizieren sich mit
Client-Credentials-Tokens ihrer eigenen, über die Registry ausgestellten
OAuth2-Clients; ihre Rollen liest das Backend anhand der Client-ID aus
der Registry, nie aus Token-Claims. Öffentlich (ohne Login) sind die
well-known-Endpunkte, der C2PA-Manifestabruf, die
Semantic-Hub-Auflösungsendpunkte sowie die von Wallets und
Peer-Instanzen aufgerufenen Callback-Pfade.

## A.1 HTTP-API-Gruppen

| Gruppe | Basispfad | Zweck |
| --- | --- | --- |
| Auth | `/auth/...` | Endnutzer-Login: OIDC-Session-Management und OpenID4VP-Presentation (inkl. PID-Presentation) |
| TemplateRepository | `/template/...` | Lebenszyklus der Vertragsvorlagen |
| SemanticHub | `/semantic/...` | Versionierte JSON-LD-Kontexte, SHACL-Shapes, Ontologien, Validierungsprofile, Klausel-Katalog |
| TemplateCatalogueIntegration | `/catalogue/template/...` | Lesende Anbindung an den XFSC Federated Catalogue |
| ContractWorkflowEngine | `/contract/...`, `/machine-identities/...` | Vertragslebenszyklus; Zielsystem-Registry und Maschinen-Identitäten |
| SignatureManagement | `/signature/...` | Signatur-Ceremonies, Prüfung, Provenance, Compliance |
| PDFGeneration | `/pdf/...`, `/contract/export/...`, `/template/export/...` | Export und unabhängige Verifikation der PDF-Repräsentationen |
| ContractStorageArchive | `/archive/...` | Langzeitarchiv abgeschlossener Verträge, Löschung und Löschstatus |
| ProcessAuditAndCompliance | `/pac/...` | Audit-Abruf, Reports, Monitoring, Merkle-Checkpoints, Workflow-Gate-Läufe |
| DcsToDcs | `/peer/...` | Föderation: PDF-/JAdES-Austausch und Löschanforderung zwischen DCS-Instanzen |
| KeyInventory | `/admin/hsm-keys` | Lesendes Inventar der HSM-Schlüssel (Sys. Administrator) |
| DIDService | `/.well-known/...` | DID-Dokument, Agreement Credential, Föderationsregeln (öffentlich) |
| C2PAService | `/c2pa/...` | Öffentlicher Abruf des C2PA-Manifests eines Vertrags |
| Webhook-Plattform | `/orce/...` | Abonnements, Zustellungen und Callbacks für externe Ereignisempfänger |

### Auth (`/auth/...`)

Zwei Ablauffamilien: der OIDC-Login mit OpenID4VP-Presentation
(Session-basierte Anmeldung am DCS) und die davon unabhängige, einmalige
PID-Presentation (Identitätsnachweis ohne Session).

| Endpunktgruppe | Endpunkte | Zweck |
| --- | --- | --- |
| Login-Flow | `POST /auth/login`, `POST /auth/login/renew`, `POST /auth/login/challenge`, `GET /auth/login/status` | OID4VP-Login starten/verlängern, Login-Challenge binden, Status-Polling für das Frontend |
| OIDC-Session | `GET /auth/consent`, `GET /auth/callback`, `POST /auth/refresh`, `GET /auth/logout`, `GET /auth/logout-complete` | Consent und Callback, Token-Refresh über HttpOnly-Cookie, Abmeldung |
| Wallet-Presentation | `GET/POST /auth/presentation/request/{state}`, `POST /auth/presentation/callback` | Signiertes Authorization-Request-Objekt (Request-by-Reference) für die Wallet; Direct-Post-Rückkanal |
| PID-Presentation | `POST /auth/pid/presentation`, `POST /auth/pid/presentation/renew`, `GET/POST /auth/pid/presentation/request/{state}`, `POST /auth/pid/presentation/callback`, `GET /auth/pid/presentation/status` | Einmalige PID-Vorlage ohne Session, gleiche Request-/Callback-/Polling-Mechanik |

### TemplateRepository (`/template/...`)

| Endpunktgruppe | Endpunkte | Zweck |
| --- | --- | --- |
| Anlage und Pflege | `POST /template/create`, `POST /template/copy`, `PUT /template/update`, `POST /template/update_manage` | Vorlage anlegen, kopieren, inhaltlich bzw. verwaltend ändern |
| Freigabeprozess | `POST /template/submit`, `POST /template/verify`, `POST /template/approve`, `POST /template/reject` | Zur Prüfung einreichen, semantisch verifizieren, freigeben oder ablehnen |
| Abruf und Suche | `GET /template/search`, `GET /template/retrieve`, `GET /template/retrieve/{did}`, `GET /template/{did}`, `GET /template/history/{did}` | Vorlagen suchen, listen, per DID auflösen, Historie einsehen |
| Provenance und Audit | `GET /template/provenance/{did}`, `GET /template/audit` | Herkunftsnachweis und Audit-Einträge einer Vorlage |
| Verteilung | `POST /template/register`, `POST /template/publish`, `POST /template/archive` | Version registrieren, im Federated Catalogue veröffentlichen, ausmustern |

### SemanticHub (`/semantic/...`)

Sämtliche lesenden Endpunkte dieser Gruppe sind öffentlich: Erzeugte
Artefakte tragen Hub-Anker, die externe Prüfer ohne DCS-Login auflösen
können müssen. Schreibend ist nur die Schema-Verwaltung, und die
ausschließlich für den Template Manager.

| Endpunktgruppe | Endpunkte | Zweck |
| --- | --- | --- |
| Schema-Verwaltung | `POST /semantic/schema/register`, `POST /semantic/schema/rollback` | Neue Schema-Version registrieren, auf eine frühere zurücksetzen (Template Manager) |
| Schema-Abruf | `GET /semantic/schema/retrieve`, `GET /semantic/schema/versions`, `GET /semantic/schema/list` | Aktive oder benannte Version abrufen, Versions- und Schemaliste |
| Artefakt-Auflösung | `GET /semantic/context/{name}`, `GET /semantic/shapes/{name}`, `GET /semantic/ontology/{name}`, `GET /semantic/profile/{name}` | Kontext, Shapes, geparste RDF-Konfiguration und Validierungsprofil unter stabilen Ankern ausliefern |
| Klausel-Katalog | `GET /semantic/clauses` | Klauseltypen für die Palette des Vorlagen-Builders |

### TemplateCatalogueIntegration (`/catalogue/template/...`)

| Endpunkte | Zweck |
| --- | --- |
| `GET /catalogue/template/retrieve`, `GET /catalogue/template/retrieve/{did}`, `GET /catalogue/template/search` | Im Federated Catalogue veröffentlichte Vorlagen listen, per DID abrufen und durchsuchen (menschliche Vertragsrollen) |

### ContractWorkflowEngine (`/contract/...`, `/machine-identities/...`)

| Endpunktgruppe | Endpunkte | Zweck |
| --- | --- | --- |
| Anlage und Pflege | `POST /contract/create`, `PUT /contract/update`, `POST /contract/renew` | Vertrag aus Vorlage anlegen, im Entwurf ändern, Folgevertrag ableiten |
| Angebot | `POST /contract/offer`, `POST /contract/withdraw`, `POST /contract/submit` | Der Gegenpartei anbieten, vor Freigabe zurückziehen, Zustandsübergang einreichen |
| Verhandlung | `POST /contract/negotiate`, `PUT /contract/negotiation_draft`, `GET/DELETE /contract/negotiation_draft/{did}`, `POST /contract/respond` | Change Requests stellen, Verhandlungsentwurf zwischenspeichern, Gegenvorschlag annehmen/ablehnen |
| Entscheidung | `POST /contract/approve`, `POST /contract/reject`, `GET /contract/review` | Freigeben oder ablehnen, Review-Sicht der offenen Aufgaben |
| Abruf und Suche | `GET /contract/retrieve`, `GET /contract/retrieve/{did}`, `GET /contract/{did}`, `GET /contract/history/{did}`, `GET /contract/search`, `GET /contract/templates` | Verträge listen, per DID auflösen (parteibeschränkt), Historie, Suche, verfügbare Vorlagen |
| Abschluss und Betrieb | `POST /contract/store`, `POST /contract/terminate`, `GET /contract/kpis/{did}`, `POST /contract/audit` | Evidenz sichern, beenden, KPI-Beobachtungen einsehen, Audit-Auszug |
| Deployment | `POST /contract/deploy`, `POST /contract/target/designate`, `POST /contract/deployment/callback` | An das designierte (oder ein explizit gewähltes) Zielsystem ausrollen; Designation setzen oder löschen; Rückmeldung des Zielsystems (Ack/Status, KPI-Werte), authentifiziert als dessen eigener OAuth2-Client und nur für Deployments, die an genau dieses Ziel gingen |
| Zielsystem-Registry | `GET/POST/PUT/DELETE /contract/targets`, `POST /contract/targets/{id}/credential` | Registry administrieren (Schreibzugriff Sys. Administrator, Lesen auch Contract Manager); Löschen wird verweigert, solange ein Vertrag das Ziel designiert; Callback-Credential ausstellen/rotieren — das Secret erscheint genau einmal, das vorherige verliert sofort seine Gültigkeit |
| Maschinen-Identitäten | `GET/POST /machine-identities`, `PUT/DELETE /machine-identities/{id}`, `POST /machine-identities/{id}/credential` | Registry verwalten: Anlegen provisioniert den OAuth2-Client und liefert das Secret genau einmal; Deaktivieren weist Aufrufe sofort ab; Löschen entfernt auch den Client; Rotation invalidiert das alte Secret unmittelbar (Sys. Administrator) |

### SignatureManagement (`/signature/...`)

| Endpunktgruppe | Endpunkte | Zweck |
| --- | --- | --- |
| Wallet-Ceremony | `POST /signature/request`, `GET /signature/request/{ceremony_id}`, `POST /signature/request/{ceremony_id}/publish`, `GET/POST /signature/request/{ceremony_id}/object`, `GET /signature/request/{ceremony_id}/document`, `GET /signature/request/{ceremony_id}/payload`, `POST /signature/request/{ceremony_id}/callback` | Ceremony starten, Status abfragen, Request-Objekt, zu signierendes PDF und kanonische JSON-LD-Payload an die Wallet ausliefern, Ergebnis über den nonce-gebundenen Callback entgegennehmen |
| Direkter Signaturfluss | `POST /signature/prepare`, `POST /signature/submit` | Vorbereitetes, byte-gepinntes PDF ausliefern und fertige externe Signatur einreichen |
| Prüfung | `POST /signature/verify`, `POST /signature/validate`, `POST /signature/compliance` | Integrität und Provenance verifizieren, DSS-gestützte Validierung, Niveau-Prüfung |
| Abruf und Nachweis | `GET /signature/retrieve`, `GET /signature/retrieve/{did}`, `GET /signature/provenance/{did}`, `GET /signature/view`, `GET /signature/audit` | Signierliste, Signaturumschlag je Vertrag, Provenance-Kette, Compliance-Viewer mit Detail je Signatur, Audit-Einträge |
| Verwaltung | `POST /signature/revoke` | Signatur widerrufen |

### PDFGeneration (`/pdf/...`, Export-Bundles)

| Endpunktgruppe | Endpunkte | Zweck |
| --- | --- | --- |
| PDF-Export | `GET /pdf/export/contract/{did}`, `GET /pdf/export/template/{did}` | Deterministisch gerendertes PDF eines Vertrags bzw. einer Vorlage |
| Bundle-Export | `GET /contract/export/{did}`, `GET /template/export/{did}` | ZIP-Bundle mit PDF und maschinenlesbaren Artefakten |
| Verifikation | `GET /pdf/verify/contract/{did}`, `GET /pdf/verify/template/{did}` | Unabhängiger Neurender-Abgleich mit benannter Fehlerklasse |

### ContractStorageArchive (`/archive/...`)

| Endpunkte | Zweck |
| --- | --- |
| `POST /archive/store` | Abgeschlossenen Vertrag mit Evidenz archivieren |
| `GET /archive/retrieve`, `GET /archive/search` | Archiveinträge abrufen und strukturiert durchsuchen (Volltext, Name, Zustand, Partei, Gültigkeitszeitraum, Annotations-Tag) |
| `POST /archive/annotate` | Eintrag mit Zusammenfassung/Tags versehen |
| `DELETE /archive/delete` | Eintrag mit Begründung löschen: Soft-Delete des Eintrags, Entfernen der Snapshots und Vernichtung der Inhaltsschlüssel auf beiden Instanzen (ADR-28) |
| `GET /archive/erasure-status` | Stand der Schlüsselvernichtung: lokale Zerstörungsnachweise und offene Peer-Anforderungen |
| `GET /archive/statistics` | Archiv-Dashboard: Bestands- und Speicherstatistik, letzte Archivaktionen, demnächst auslaufende Verträge, Evidenz-Vollständigkeit |
| `GET /archive/audit` | Audit-Einträge des Archivs |

### ProcessAuditAndCompliance (`/pac/...`)

| Endpunkte | Zweck |
| --- | --- |
| `POST /pac/audit` | Audit-Abruf: Evidenz sammeln und extern bewerten lassen (Begründung Pflicht, Abruf wird auditiert) |
| `GET /pac/report` | Audit-Report erzeugen (JSON/CSV/PDF) |
| `POST /pac/report` | Incident-Report zu einer Ressource erfassen |
| `GET /pac/monitor` | Compliance-Sweep auf Anfrage; auch ein Lauf ohne Befund wird auditiert |
| `GET /pac/audit/checkpoint/head`, `GET /pac/audit/checkpoint/{seq}`, `GET /pac/audit/checkpoint/proof/{entry_cid}` | Aktueller Merkle-Checkpoint-Kopf, ein bestimmter Checkpoint, Inklusionsbeweis für einen Audit-Eintrag |
| `GET /pac/workflow-gates/{run_id}`, `POST /pac/workflow-gates/review` | Workflow-Gate-Lauf lesen; eine `REVIEW`-Entscheidung mit Begründung festhalten |

### DcsToDcs (`/peer/...`)

Föderationsschnittstelle zwischen zwei DCS-Instanzen. Alle Endpunkte
stehen unter dem Trust Gate (Agreement Credential plus lokaler
Policy-Endpunkt, fail-closed) und werden per did:web-Challenge-Response
im Request-Body authentifiziert, nicht per Session-Token.

| Endpunkte | Zweck |
| --- | --- |
| `POST /peer/contracts/pdf` | Eingang eines Vertrags-PDFs mit Zustand, optional JAdES, Vollmachtsnachweis und gewickeltem Inhaltsschlüssel |
| `POST /peer/contracts/erase` | Löschanforderung: Vernichtung der Inhaltsschlüssel dieses Vertrags auf der empfangenden Instanz |
| `GET /peer/contracts/provenance` | Gespeicherte JAdES-Provenienz eines empfangenen Vertrags |

### KeyInventory (`/admin/hsm-keys`)

| Endpunkt | Zweck |
| --- | --- |
| `GET /admin/hsm-keys` | Lesendes Inventar: Label, Zweck und aktive Version jedes HSM-Schlüssels der Instanz (Sys. Administrator). Rotation selbst ist ein Betriebsverfahren, keine API-Handlung |

### C2PAService (`/c2pa/...`)

| Endpunkt | Zweck |
| --- | --- |
| `GET /c2pa/manifest/{contract_did}` | Öffentlicher, nicht authentifizierter Abruf des C2PA-Manifest-Stores eines Vertrags; optional die Kettenaufzählung |

## A.2 Ereignisse auf dem Event-Bus

Alle Backend-Domänen publizieren ihre Ereignisse als CloudEvents über
einen gemeinsamen NATS-Topic. Die Zustellung folgt dem
Transactional-Outbox-Muster: Der Handler persistiert das Ereignis in
derselben Datenbanktransaktion wie seine Änderung; ein Outbox-Prozessor
publiziert es auf dem Bus und verankert es als hash-verketteten
Audit-Trail-Eintrag. Konsumenten unterscheiden Ereignisse über den
CloudEvent-Typ, nicht über Subtopics.

Abonnenten innerhalb der Instanz:

| Konsument | Reagiert auf | Wirkung |
| --- | --- | --- |
| DCS-to-DCS-Synchronizer | Lebenszyklus- und Remote-Sync-Ereignisse, `PDF_REGENERATED` | Versand des Vertrags-PDFs an die Peer-Instanz bei versandpflichtigen Zuständen |
| PDF-/C2PA-Regenerator | Lebenszyklusereignisse von Verträgen und Vorlagen | Hintergrund-Neurendering und Anfügen einer C2PA-Lifecycle-Assertion — Exporte werden nie on demand erzeugt |
| Webhook-Plattform | Domänenereignisse mit Webhook-Abbildung | Weiterleitung an registrierte externe Abonnenten |
| Auto-Deployment | `APPLIED_SIGNATURE` | Anstoß der Zielsystem-Auslieferung über dasselbe Kommando wie der manuelle Deploy |
| Event-Logger (optional) | alle Ereignisse | Diagnose-Mitschnitt, nur wenn ausdrücklich eingeschaltet |

Ereignistypen je Domäne (CloudEvent-Typ auf dem Bus; zugleich der
Ereignistyp im Audit-Trail):

| Domäne | Ereignistypen |
| --- | --- |
| Contract Workflow Engine | `CREATE_CONTRACT`, `UPDATE_CONTRACT`, `SUBMIT_CONTRACT`, `OFFER_CONTRACT`, `WITHDRAW_CONTRACT`, `NEGOTIATE_CONTRACT`, `ACCEPT_RESPOND_CONTRACT`, `REJECT_RESPOND_CONTRACT`, `INCREASE_CONTRACT_VERSION`, `APPROVE_CONTRACT`, `REJECT_CONTRACT`, `VERIFY_CONTRACT`, `REVIEW_CONTRACT`, `TERMINATE_CONTRACT`, `RENEW_CONTRACT`, `REVOKE_CONTRACT`, `CONTRACT_EXPIRED`, `RECORD_EVIDENCE`, `AUDIT_CONTRACT`, `EXPORT`, `SEARCH_CONTRACT`, `RETRIEVE_ALL_CONTRACTS`, `RETRIEVE_CONTRACT_BY_ID`, `RETRIEVE_CONTRACT_HISTORY_BY_DID`, `RETRIEVE_ALL_TEMPLATES`, `CONTRACT_ACCESS_DENIED` |
| Föderation und Synchronisation | `REMOTE_SYNC`, `REMOTE_SYNC_REQUEST`, `REMOTE_ACTION_REQUEST`, `OUTDATED_PEER`, `PDF_REGENERATED` |
| Archiv und Löschung | `STORE_ARCHIVED_CONTRACT`, `RETRIEVE_ARCHIVED_CONTRACTS`, `DELETE_ARCHIVED_CONTRACT`, `ANNOTATE_ARCHIVED_CONTRACT`, `KEY_SHREDDED` |
| Template Repository | `CREATE_CONTRACT_TEMPLATE`, `COPY_CONTRACT_TEMPLATE`, `UPDATE_CONTRACT_TEMPLATE`, `SUBMIT_CONTRACT_TEMPLATE`, `VERIFY_CONTRACT_TEMPLATE`, `APPROVE_CONTRACT_TEMPLATE`, `REJECT_CONTRACT_TEMPLATE`, `REGISTER_CONTRACT_TEMPLATE`, `PUBLISH_CONTRACT_TEMPLATE`, `ARCHIVE_CONTRACT_TEMPLATE`, `AUDIT_CONTRACT_TEMPLATE`, `SEARCH_CONTRACT_TEMPLATE`, `RETRIEVE_ALL_CONTRACT_TEMPLATES`, `RETRIEVE_CONTRACT_TEMPLATE_BY_ID` |
| Signature Management | `SIGNING_REQUEST`, `APPLY_SIGNATURE`, `APPLIED_SIGNATURE`, `VERIFY_SIGNATURE`, `VALIDATE_SIGNATURE`, `REVOKE_SIGNATURE`, `COMPLIANCE_VALIDATION` |
| Process Audit & Compliance | `PAC_AUDIT_EXECUTED`, `PAC_REPORT_GENERATED`, `PAC_COMPLIANCE_MONITOR`, `PAC_COMPLIANCE_RISK`, `PAC_INCIDENT_REPORT`, `PAC_TRUST_GATE_DENIAL`, `CONFIG_INTEGRITY_ATTESTATION`, `TEMPLATE_POLICY_AUDIT_FINDING`, `TEMPLATE_APPROVAL_PROVENANCE_AUDIT_FINDING`, `CONTRACT_CONTENT_POLICY_AUDIT_FINDING` |
| Catalogue Integration | `RETRIEVE_ALL_TEMPLATE_CATALOGUE`, `RETRIEVE_TEMPLATE_CATALOGUE_BY_ID`, `SEARCH_TEMPLATE_CATALOGUE` |
| Auth | `OID4VP_PRESENTATION_SUCCEEDED`, `OID4VP_PRESENTATION_FAILED` |

### Webhook-Plattform (`/orce/...`)

Die Webhook-Plattform liegt außerhalb der generierten API und übersetzt
interne Ereignisse in benannte, abonnierbare Alert-Ereignisse.

Abonnierbare Ereignisse: `contract.created`, `contract.submitted`,
`contract.approved`, `contract.rejected`, `contract.negotiated`,
`contract.terminated`, `contract.expired`, `template.created`,
`template.approved`, `template.updated`, `template.registered`,
`template.deprecated`, `compliance.risk_detected`.

| Endpunkt | Zweck |
| --- | --- |
| `GET /orce/events` | Katalog der abonnierbaren Ereignisnamen |
| `GET /orce/webhooks`, `POST /orce/webhooks`, `DELETE /orce/webhooks/{id}` | Subscriptions listen, registrieren (Ereignis, Callback-URL, Secret), löschen |
| `POST /orce/callbacks` | Quittung des Abonnenten für eine Zustellung |
| `GET /orce/deliveries` | Zustellprotokoll (Ergebnis, Statuscode, Quittungsstatus) |

Subscriptions und Zustellprotokoll liegen im Prozessspeicher und
überdauern keinen Neustart (Kapitel 10).

## A.3 Well-known-Endpunkte

Öffentliche, nicht authentifizierte Endpunkte an der Origin-Wurzel der
Instanz; dieselben Dokumente sind zusätzlich unter dem API-Präfix
erreichbar. Sie sind die Auflösungsbasis des Föderations-Trust-Modells:
Peer-Instanzen und externe Prüfer holen sich hier Identität,
Regelakzeptanz und Regeln einer Instanz, bevor Vertragsdaten fließen.

| Endpunkt | Inhalt | Zweck |
| --- | --- | --- |
| `GET /.well-known/did.json` | DID-Dokument der Instanz (did:web) | Instanzidentität; öffentliche Schlüssel zu den im HSM gehaltenen privaten Schlüsseln, einschließlich des Schlüsselvereinbarungs-Schlüssels |
| `GET /.well-known/dcs-agreement-credential.json` | Selbstsigniertes Agreement Credential | Nachweis, dass die Instanz die Föderationsregeln akzeptiert hat — Prüfgegenstand des Trust Gates auf Ein- und Ausgangspfad |
| `GET /.well-known/dcs-federation-rules.md` | Föderationsregeln | Das Regeldokument, auf das sich das Agreement Credential per Hash bezieht |

Ergänzend zur Auflösung durch Dritte:

- **OID4VP-Metadaten** werden nicht als eigenes well-known-Dokument
  ausgeliefert: Die Verifier-Metadaten stecken im signierten
  Authorization-Request-Objekt, das die Wallet per Request-by-Reference
  abruft; der Verifikationsschlüssel wird über die im JAR-Header
  mitgelieferte Zertifikatskette und den DNS-gebundenen Client-Identifier
  verankert.
- Die **OIDC-Discovery** liefert nicht das DCS-Backend, sondern der
  mitbetriebene OAuth2-Provider.
- `GET /c2pa/manifest/{contract_did}` ist das öffentliche Gegenstück für
  die Provenance-Auflösung eines Vertrags.
- Der **Metrik-Endpunkt** liegt außerhalb der authentifizierten API und
  außerhalb des Anfragebudgets (Kapitel 10).
