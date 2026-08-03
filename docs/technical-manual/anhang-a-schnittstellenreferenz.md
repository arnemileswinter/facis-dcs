# Anhang A Schnittstellenreferenz

Dieser Anhang listet die Schnittstellen einer DCS-Instanz auf drei
Ebenen: die HTTP-API-Gruppen des Backends, die Ereignisse auf dem
internen Event-Bus und die öffentlichen well-known-Endpunkte. Er
beschreibt Gruppen und Zweck, nicht einzelne Parameter; die
vollständige, maschinenlesbare API-Beschreibung liefert jede laufende
Instanz selbst (Swagger-Oberfläche und OpenAPI-3-Spezifikation).

Alle fachlichen APIs sind, sofern nicht anders vermerkt, durch das
OIDC-Bearer-Token geschützt; die Scopes im Access Token entsprechen den
Rollen-Claims des Nutzers. Maschinelle Aufrufer authentifizieren sich
mit Client-Credentials-Tokens ihrer eigenen OAuth2-Clients; ihre Rollen
liest das Backend anhand der Client-ID aus der Registry, nie aus
Token-Claims. Öffentlich sind die well-known-Endpunkte, der
C2PA-Manifestabruf, die Semantic-Hub-Auflösung sowie die von Wallets und
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

Zwei Ablauffamilien: der OIDC-Login mit OpenID4VP-Presentation und die
davon unabhängige, einmalige PID-Presentation ohne Session.

| Endpunktgruppe | Endpunkte | Zweck |
| --- | --- | --- |
| Login und OIDC-Session | `POST /auth/login`, `/auth/login/renew`, `/auth/login/challenge`, `GET /auth/login/status`; `GET /auth/consent`, `/auth/callback`, `POST /auth/refresh`, `GET /auth/logout`, `/auth/logout-complete` | OID4VP-Login starten, verlängern und an die Hydra-Challenge binden; Status-Polling; Consent und Callback; Token-Refresh über HttpOnly-Cookie; Abmeldung |
| Wallet- und PID-Presentation | `GET/POST /auth/presentation/request/{state}`, `POST /auth/presentation/callback`; gleiche Mechanik sessionlos unter `/auth/pid/presentation/…` | Signiertes Authorization-Request-Objekt (Request-by-Reference) und Direct-Post-Rückkanal; die PID-Variante ohne Session (Kapitel 05) |

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
können müssen. Schreibend ist nur die Schema-Verwaltung (Template
Manager).

| Endpunktgruppe | Endpunkte | Zweck |
| --- | --- | --- |
| Schema-Verwaltung | `POST /semantic/schema/register`, `POST /semantic/schema/rollback` | Neue Schema-Version registrieren, auf eine frühere zurücksetzen |
| Schema-Abruf | `GET /semantic/schema/retrieve`, `GET /semantic/schema/versions`, `GET /semantic/schema/list` | Aktive oder benannte Version abrufen, Versions- und Schemaliste |
| Artefakt-Auflösung | `GET /semantic/context/{name}`, `GET /semantic/shapes/{name}`, `GET /semantic/ontology/{name}`, `GET /semantic/profile/{name}` | Kontext, Shapes, RDF-Konfiguration und Validierungsprofil unter stabilen Ankern |
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
| Deployment | `POST /contract/deploy`, `POST /contract/target/designate`, `POST /contract/deployment/callback` | An das designierte (oder ein explizit gewähltes) Zielsystem ausrollen; Designation setzen oder löschen; Rückmeldung des Zielsystems (Ack/Status, KPI-Beobachtungen samt Urteil und Regelbezug, ADR-33), authentifiziert als dessen eigener OAuth2-Client und nur für Deployments an genau dieses Ziel |
| Zielsystem-Registry | `GET/POST/PUT/DELETE /contract/targets`, `POST /contract/targets/{id}/credential` | Registry administrieren (Schreibzugriff Sys. Administrator und Integration Manager, Lesen zusätzlich Contract Manager); Löschen wird verweigert, solange ein Vertrag das Ziel designiert; Callback-Credential ausstellen/rotieren (Secret genau einmal sichtbar) |
| Maschinen-Identitäten | `GET/POST /machine-identities`, `PUT/DELETE /machine-identities/{id}`, `POST /machine-identities/{id}/credential` | Registry verwalten: Anlegen provisioniert den OAuth2-Client und liefert das Secret genau einmal; Deaktivieren weist Aufrufe sofort ab; Löschen entfernt auch den Client; Rotation invalidiert das alte Secret unmittelbar (Sys. Administrator) |

### SignatureManagement (`/signature/...`)

| Endpunktgruppe | Endpunkte | Zweck |
| --- | --- | --- |
| Wallet-Ceremony | `POST /signature/request`, `GET /signature/request/{ceremony_id}`, `POST /signature/request/{ceremony_id}/publish`, `GET/POST /signature/request/{ceremony_id}/object`, `GET /signature/request/{ceremony_id}/document`, `GET /signature/request/{ceremony_id}/payload`, `POST /signature/request/{ceremony_id}/callback` | Ceremony starten, Status abfragen, Request-Objekt, zu signierendes PDF und kanonische JSON-LD-Payload an die Wallet ausliefern, Ergebnis über den nonce-gebundenen Callback entgegennehmen |
| Direkter Signaturfluss | `POST /signature/prepare`, `POST /signature/submit` | Vorbereitetes, byte-gepinntes PDF ausliefern und fertige externe Signatur einreichen |
| Prüfung | `POST /signature/verify`, `POST /signature/validate`, `POST /signature/compliance` | Integrität und Provenance verifizieren, DSS-gestützte Validierung, Niveau-Prüfung |
| Abruf und Nachweis | `GET /signature/retrieve`, `GET /signature/retrieve/{did}`, `GET /signature/provenance/{did}`, `GET /signature/view`, `GET /signature/audit` | Signierliste, Signaturumschlag je Vertrag, Provenance-Kette, Compliance-Viewer, Audit-Einträge |
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
| `DELETE /archive/delete` | Eintrag mit Begründung löschen: Soft-Delete, Entfernen der Snapshots und Vernichtung der Inhaltsschlüssel auf beiden Instanzen (ADR-28) |
| `GET /archive/erasure-status` | Stand der Schlüsselvernichtung: lokale Zerstörungsnachweise und offene Peer-Anforderungen |
| `GET /archive/statistics` | Archiv-Dashboard: Bestands- und Speicherstatistik, letzte Aktionen, demnächst auslaufende Verträge, Evidenz-Vollständigkeit |
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
| `GET /peer/contracts/provenance` | Gespeicherte JAdES-Provenienz eines empfangenen Vertrags (JWT-geschützt, für lokale Nutzer) |

### KeyInventory (`/admin/hsm-keys`)

| Endpunkt | Zweck |
| --- | --- |
| `GET /admin/hsm-keys` | Lesendes Inventar: Label, Zweck und aktive Version jedes HSM-Schlüssels (Sys. Administrator). Rotation ist ein Betriebsverfahren, keine API-Handlung |

### C2PAService (`/c2pa/...`)

| Endpunkt | Zweck |
| --- | --- |
| `GET /c2pa/manifest/{contract_did}` | Öffentlicher, nicht authentifizierter Abruf des C2PA-Manifest-Stores eines Vertrags; optional die Kettenaufzählung |

## A.2 Ereignisse auf dem Event-Bus

Alle Backend-Domänen publizieren ihre Ereignisse als CloudEvents über
einen gemeinsamen NATS-Topic, zugestellt nach dem
Transactional-Outbox-Muster (Kapitel 09). Konsumenten unterscheiden
Ereignisse über den CloudEvent-Typ.

Abonnenten innerhalb der Instanz:

| Konsument | Reagiert auf | Wirkung |
| --- | --- | --- |
| DCS-to-DCS-Synchronizer | `OFFER_CONTRACT`, `PDF_REGENERATED`, `APPLIED_SIGNATURE`, `REVOKE_SIGNATURE` | Versand des Vertrags-PDFs an die Peer-Instanz bei versandpflichtigen Zuständen |
| PDF-/C2PA-Regenerator | Lebenszyklusereignisse von Verträgen und Vorlagen | Hintergrund-Neurendering und Anfügen einer C2PA-Lifecycle-Assertion |
| Webhook-Plattform | Domänenereignisse mit Webhook-Abbildung | Weiterleitung an registrierte externe Abonnenten |
| Auto-Deployment | `APPLIED_SIGNATURE` | Anstoß der Zielsystem-Auslieferung über dasselbe Kommando wie der manuelle Deploy |
| Event-Logger (optional) | alle Ereignisse | Diagnose-Mitschnitt, nur wenn ausdrücklich eingeschaltet |

Die CloudEvent-Typen folgen dem Kommando-Vokabular ihrer Domäne
(`CREATE_CONTRACT`, `PUBLISH_CONTRACT_TEMPLATE`, `APPLIED_SIGNATURE`,
`KEY_SHREDDED`, `PAC_COMPLIANCE_RISK`, …) und sind zugleich der
Ereignistyp im Audit-Trail. Maßgeblich ist der Typ auf dem Bus; reine
Lesezugriffe erzeugen Lookup-Ereignisse, die nicht im Trail verankert
werden (Kapitel 09).

### Webhook-Plattform (`/orce/...`)

Die Webhook-Plattform übersetzt interne Ereignisse in benannte,
abonnierbare Alert-Ereignisse: `contract.created`, `contract.submitted`,
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
erreichbar. Sie sind die Auflösungsbasis des Föderations-Trust-Modells.

| Endpunkt | Inhalt | Zweck |
| --- | --- | --- |
| `GET /.well-known/did.json` | DID-Dokument der Instanz (did:web) | Instanzidentität; öffentliche Schlüssel zu den im HSM gehaltenen privaten Schlüsseln, einschließlich des Schlüsselvereinbarungs-Schlüssels |
| `GET /.well-known/dcs-agreement-credential.json` | Selbstsigniertes Agreement Credential | Nachweis der Regelakzeptanz; Prüfgegenstand des Trust Gates auf Ein- und Ausgangspfad |
| `GET /.well-known/dcs-federation-rules.md` | Föderationsregeln | Das Regeldokument, auf das sich das Agreement Credential per Hash bezieht |

Ergänzend zur Auflösung durch Dritte:

- **OID4VP-Metadaten** werden nicht als eigenes well-known-Dokument
  ausgeliefert: Die Verifier-Metadaten stecken im signierten
  Authorization-Request-Objekt, verankert über die Zertifikatskette im
  JAR-Header und den DNS-gebundenen Client-Identifier.
- Die **OIDC-Discovery** liefert nicht das DCS-Backend, sondern der
  mitbetriebene OAuth2-Provider.
- `GET /c2pa/manifest/{contract_did}` ist das öffentliche Gegenstück für
  die Provenance-Auflösung eines Vertrags.
- Der **Metrik-Endpunkt** liegt außerhalb der authentifizierten API und
  außerhalb des Anfragebudgets (Kapitel 10).
