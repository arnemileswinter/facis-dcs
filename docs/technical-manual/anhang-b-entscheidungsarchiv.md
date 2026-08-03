# Anhang B Entscheidungsarchiv

Verweisliste auf die Architecture Decision Records unter `docs/`. Jeder
Eintrag nennt die These des ADRs und, wo relevant, seine Beziehung zu
anderen ADRs. Begründungen und Konsequenzen stehen ausschließlich im
jeweiligen ADR. ADRs ohne abweichenden Vermerk sind unverändert in Kraft.

## Projektreihe

| ADR | These und Status |
| --- | --- |
| [ADR-1](../adr-1-key-custody.md) | PKCS#11/HSM ist der einzige Key-Custody-Mechanismus für alle DCS-eigenen Signaturschlüssel. In Teilen superseded durch ADR-12: PAdES-Vertragssignaturen zählen nicht zu den DCS-Key-Custody-Touchpoints. |
| [ADR-2](../adr-2-contract-state-machine.md) | Eine einzige Vertrags-State-Machine mit einer Transitionstabelle. Der ursprünglich implizierte Full-State-Broadcast-Synchronizer ist durch ADR-13 zurückgezogen. |
| [ADR-3](../adr-3-signing-semantics.md) | Signatursemantik: Identitätsbindung unter der Signatur, Embed-then-Sign. Die org-key-basierte AES über das PDF ist durch ADR-12 und ADR-20 zurückgezogen. |
| [ADR-4](../adr-4-c2pa-embedding.md) | C2PA-Provenance wird doppelt ausgeliefert: eingebettet als JUMBF im PDF und als standardkonformes Remote-Manifest. |
| [ADR-5](../adr-5-xfsc-component-posture.md) | Festlegung je XFSC-Komponente, ob sie übernommen oder substituiert wird; schließt Graph-Store und Deployment-Lifecycle des übernommenen Federated Catalogue ein. |
| [ADR-6](../adr-6-odrl-profile-enforcement.md) | Vertragsbedingungen sind echtes ODRL unter einem DCS-Profil und werden serverseitig durchgesetzt. Nur der Evaluator-Mechanismus ist durch ADR-11 superseded. |
| [ADR-7](../adr-7-hierarchy-direction.md) | Vertragshierarchie-Links zeigen ausschließlich vom Kind zum Elternvertrag, nie umgekehrt. |
| [ADR-8](../adr-8-semantic-hub-version-pinning.md) | Enforcement bezieht SHACL-Shapes und Validierungsprofil ausschließlich aus versionsgepinnten Semantic-Hub-Ständen. |
| [ADR-9](../adr-9-shacl-engine-gordflib.md) | Festlegung der SHACL-Engine für die semantische Validierung. |
| [ADR-10](../adr-10-clause-catalog-transport.md) | Der Klausel-Katalog wird als vorverdautes JSON zusammen mit dem rohen Turtle in einer Antwort transportiert. |
| [ADR-11](../adr-11-opa-odrl-enforcement.md) | ODRL-Constraints werden auf eingebettetem OPA evaluiert. Supersedet den Evaluator-Mechanismus aus ADR-6; das Ausführungs-/KPI-Moment ist seinerseits durch ADR-33 superseded, das Vor-Signatur-Moment steht unverändert. |
| [ADR-12](../adr-12-wallet-driven-signing.md) | Die Vertragssignatur ist wallet-getrieben: Das DCS ist OpenID4VP-Relying-Party und Signaturvalidator und hält keinen Vertragssignaturschlüssel. Supersedet die Organisationssignatur-Klausel von ADR-3 und beschneidet ADR-1; die Akzeptanzpfad-Klauseln sind ihrerseits durch ADR-20 superseded. |
| [ADR-13](../adr-13-pdf-exchange-federation.md) | DCS-to-DCS-Föderation ist ein Austausch von Vertrags-PDF und JAdES, kein State-Broadcast; jede Instanz führt Workflow und RBAC lokal. Supersedet den Synchronizer aus ADR-2; die dritte Vertrauensschicht ist durch ADR-19 superseded. |
| [ADR-14](../adr-14-jsonld-single-expanded-form.md) | Intern soll genau eine JSON-LD-Form existieren, expandiert, mit Expansion nur an den Ingestion-Kanten. Als Entscheidung angenommen, nicht umgesetzt: Die ausgelieferte interne Form ist die kanonische `dcs:`-präfixierte Hülle (Kapitel 03). |
| [ADR-15](../adr-15-placeholder-typed-node.md) | Ein verhandelbarer Datenpunkt ist genau ein typisierter, SHACL-abgeleiteter, per `@id` verlinkter Knoten statt einer mehrstufigen Placeholder-Indirektion. Superseded durch ADR-22, das den Kern behält und den Knoten umbenennt. |
| [ADR-16](../adr-16-audit-checkpoints-external-anchoring.md) | Manipulationsnachweis des Audit-Trails entsteht über Merkle-Checkpoints je Verankerungs-Tick mit externer Verankerung. Verfeinert den event-sourced Audit-Trail, dessen Hash-Ketten je Ressource bestehen bleiben. |
| [ADR-17](../adr-17-machine-signing-is-not-an-aes-signature.md) | Eine Maschinen-Identität kann keine AES erzeugen: Die Klasse System Contract Signer erhält keinerlei Signatur-Scope. Um die Siegel-Option erweitert durch ADR-21. |
| [ADR-18](../adr-18-fc-catalogue-single-service-account.md) | Gegenüber dem Federated Catalogue tritt das DCS-Deployment als ein Participant mit einem Service-Account auf, nicht der einzelne Publisher. Ein entfernter, nicht mitdeployter Katalog erfordert eine explizite Trust-Boundary-Bestätigung, sonst schlägt das Rendering fehl. |
| [ADR-19](../adr-19-federation-agreement-credential.md) | Föderationsvertrauen ist geschichtet: did:web-Identität mit Zertifikatskette, Regelakzeptanz per Agreement Credential und ein lokaler Policy-Endpunkt (fail-closed). Ersetzt die dritte Vertrauensschicht von ADR-13. |
| [ADR-20](../adr-20-signing-acceptance-hardening.md) | Härtung des Signatur-Akzeptanzpfads: Nonce-Bindung des Ceremony-Callbacks, Byte-Pinning des zu signierenden Dokuments, atomare Ceremony-Konsumption, level-bewusste Gates, Zertifikat-zu-Identität-Prüfung, JAdES-Validierung. Supersedet die Akzeptanzpfad-Klauseln von ADR-12. |
| [ADR-21](../adr-21-system-contract-sealer.md) | System Contract Sealer: das von ADR-17 zurückgestellte elektronische Siegel über den generischen Verify-on-Reupload-Pfad statt einer Lockerung des AES-Verbots. Als Richtung akzeptiert, nicht implementiert. |
| [ADR-22](../adr-22-contract-field-model.md) | Der typisierte Knoten ist ein Feld, kein Placeholder: `dcs:contractFields` trägt die Felddeklarationen, `dcs:contractData` typisierte Domänenobjekte mit Feldreferenzen. Supersedet ADR-15; durch ADR-23 amendiert. |
| [ADR-23](../adr-23-generic-contract-data-graph.md) | Der Contract-Data-Graph ist generisch: beliebige SHACL-Vokabulare aus dem Semantic Hub, Properties als Literal, Feldreferenz oder Objektreferenz, In-Document-Auflösung und Validierung einer feld-materialisierten Kopie. Amendiert ADR-22, dessen Zwei-Ebenen-Form als Spezialfall gültig bleibt. |
| [ADR-24](../adr-24-v1-signature-scope.md) | v1-Signatur-Scope: wallet-basierte AES; das erreichte Level bestimmt der angebundene Vertrauensdienst, nicht die Codebasis. QES und weitere Formate sind als kundenbestätigte Abweichungen dokumentiert; die technischen Konformitäts-Gates bleiben erhalten. |
| [ADR-25](../adr-25-contract-target-registry.md) | Zielsysteme sind eine persistierte, administrierte Registry, und jeder Vertrag designiert sein eigenes Deployment-Ziel; ein Vertrag ohne Designation deployt nicht. Die Callback-Authentifizierung ist durch ADR-27 superseded. |
| [ADR-26](../adr-26-provenance-reanchored-after-signing.md) | Die Signatur wird über der Provenance angebracht, und die Provenance danach über der Signatur neu verankert (provenance-only C2PA-Update-Manifest als inkrementelles Update); Signaturvalidität behält in jedem Konflikt Vorrang. |
| [ADR-27](../adr-27-machine-credentials-issued-not-configured.md) | Maschinen-Credentials werden ausgestellt, nicht konfiguriert: Jeder maschinelle Aufrufer erhält einen eigenen, über den OAuth2-Provider provisionierten Client mit genau einmal angezeigtem Secret; die Deployment-Deklaration ist auf einen Seed reduziert. Supersedet die Callback-Authentifizierung aus ADR-25. |
| [ADR-28](../adr-28-ipfs-encryption-at-rest-key-shredding.md) | Jedes gespeicherte Artefakt ist unter einem zufälligen Schlüssel je Löschbereich verschlüsselt, der Schlüssel existiert im Ruhezustand nur HSM-gewickelt, und Art.-17-Löschung ist die protokollierte Vernichtung dieser Kopien auf beiden Föderationsinstanzen. |
| [ADR-29](../adr-29-poa-issuer-authority-and-integration-manager-scope.md) | Die organisatorische Autorität einer Handlungsvollmacht ist an ihren Aussteller gebunden; die Rolle Integration Manager umfasst Integrationen, nicht Zugriff. |
| [ADR-30](../adr-30-tamper-is-a-verdict-not-an-error.md) | Ein Artefakt, das die authentifizierte Entschlüsselung nicht besteht, ist ein negatives Prüfergebnis, kein Serverfehler; das Verifikationsergebnis benennt seine Fehlerklasse direkt. |
| [ADR-31](../adr-31-purpose-scoped-issuer-trust.md) | Ausstellervertrauen ist zweckgebunden (`login`/`peer`/`pid`), an Organisationen gebunden und über einen deklarierten Mechanismus aufgelöst; die Handlungsvollmacht hinter einer Signatur reist mit und wird von der Gegenseite geprüft. |
| [ADR-32](../adr-32-scheduled-compliance-monitoring.md) | Kontinuierliches Compliance-Monitoring läuft als geplanter Sweep über ein eigenes Kommando statt als Schleife im Server, mit einem Risiko-Register zur Deduplizierung der Alarme. |
| [ADR-33](../adr-33-the-target-system-observes-the-dcs-records.md) | Das Zielsystem klassifiziert die Vertragserfüllung zur Laufzeit; das DCS zeichnet die gemeldeten Urteile auf, attribuiert sie zur ODRL-Regel und auditiert sie, statt sie selbst abzuleiten. Supersedet das Ausführungs-/KPI-Moment aus ADR-11. |
| [ADR-34](../adr-34-status-lists-are-signed-by-the-issuer-that-issued-the-credential.md) | Eine Statusliste wird vom Aussteller signiert und bedient, dessen Credential sie regiert, und genauso verifiziert wie das Credential selbst; eine unsignierte Statusliste wird unter keiner Konfiguration akzeptiert. |
| [ADR (OCM-W)](../adr-ocmw-vc-signing.md) | VC-Signierung läuft direkt über das HSM, nicht über die OCM-W-Dienste. Einziger unnummerierter ADR der Reihe. |

## Backend-interne Reihe

Ältere, backend-lokale Entscheidungen unter
`docs/backend/en/decisions/`. Sie beschreiben Grundmuster, auf denen die
Projektreihe aufsetzt.

| ADR | These |
| --- | --- |
| [0001](../backend/en/decisions/0001-cqrs-per-domain.md) | CQRS-Aufteilung je Fachdomäne (command/query/db/datatype/event). |
| [0002](../backend/en/decisions/0002-goa-design-first.md) | Design-first-API mit generiertem Transport. |
| [0003](../backend/en/decisions/0003-event-sourced-audit-trail.md) | Event-sourced Audit-Trail mit Hash-Ketten je Ressource, verfeinert durch ADR-16. |
| [0004](../backend/en/decisions/0004-did-web-eidas-trust.md) | did:web-Identität mit eIDAS-Zertifikatskette als Vertrauensanker. |
| [0005](../backend/en/decisions/0005-single-writer-peer-sync.md) | Ein Schreiber je Vertrag (die Origin-Instanz); die Transportform ist durch ADR-13 abgelöst. |
| [0006](../backend/en/decisions/0006-versioning-strategies.md) | Versionierungsstrategien für Vorlagen und Verträge. |
| [0007](../backend/en/decisions/0007-optimistic-concurrency-timestamp.md) | Optimistische Nebenläufigkeitskontrolle über den Änderungszeitstempel. |
