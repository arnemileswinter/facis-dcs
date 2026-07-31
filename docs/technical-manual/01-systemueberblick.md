# 01 Systemüberblick

## Was das DCS ist

Der **Digital Contracting Service (DCS)** ist ein föderiertes,
cross-federation-fähiges und on-premise hostbares System für die sichere
B2B-Kommunikation, Verhandlung und automatisierte Verarbeitung digitaler
Verträge.

Der zentrale Gedanke: **Ein digitaler Vertrag ist nicht nur ein signiertes
Dokument, sondern eine interoperable, maschineninterpretierbare und
kryptographisch nachweisbare Vereinbarung zwischen Organisationen.**
Vertragsbedingungen können von Maschinen interpretiert, zwischen
Organisationen verhandelt, kryptographisch nachgewiesen, technisch
umgesetzt und über ihren gesamten Lebenszyklus hinweg unabhängig
auditierbar verfolgt werden.

Jede Organisation betreibt ihre eigene DCS-Instanz. Instanzen
unterschiedlicher Betreiber tauschen Vertragsvorlagen und Verträge
untereinander aus, ohne einander implizit zu vertrauen. Vertrauen wird pro
Interaktion kryptographisch hergestellt und überprüft.

## Einordnung: Gaia-X, XFSC, FACIS

Das DCS entsteht im Rahmen von **FACIS** (Federation Architecture for
Composed Infrastructure Services) innerhalb des Eclipse-XFSC-Ökosystems
(Cross Federation Services Components) und fügt sich in die
Gaia-X-Infrastrukturlandschaft ein:

- **Federated Catalogue (XFSC/Gaia-X):** Freigegebene Vertragsvorlagen
  werden als Self-Descriptions im Federated Catalogue registriert und
  publiziert und damit föderationsweit auffindbar.
- **ORCE (XFSC Orchestration Engine):** Eine Node-RED-basierte
  Workflow-Engine dient als Orchestrierungs- und Integrationsschicht, unter
  anderem für Zeitstempel, Archiv-Notariat, Zielsystem-Anbindung, die
  föderationsweite Policy-Entscheidung (Trust-PDP) sowie die auslagerbaren
  Prüf- und Freigabeschritte des Compliance-Subsystems.
- **XFSC Status List Service:** Verwaltet den Widerrufsstatus der vom
  DCS ausgestellten Lifecycle-Credentials.
- **EUDI Wallet / eIDAS 2.0:** Endnutzer authentifizieren sich über
  Verifiable Credentials (OpenID4VP), die unter anderem aus einer EUDI
  Wallet vorgelegt werden können; Signaturen folgen den
  eIDAS-Signaturformaten (PAdES/JAdES).

Die verbindliche Anforderungsbasis ist die SRS des FACIS-DCS. Die
Positionierung gegenüber den übrigen XFSC-Komponenten beschreibt ADR-5
(XFSC Component Posture).

## Kernfähigkeiten

### Semantische Vertragsvorlagen und maschinenlesbare Bedingungen

Organisationen definieren auf Basis frei wählbarer Ontologien semantisch
maschinenlesbare Vertragsvorlagen. Ein instanzlokaler **Semantic Hub**
speichert versioniert die JSON-LD-Kontexte, SHACL-Shapes und
Validierungsprofile, gegen die jedes Dokument erzeugt wird, und stellt
den Klausel-Katalog bereit. Vertragsbedingungen werden mit **ODRL 2.0**
formal beschrieben und sind dadurch für Menschen und Maschinen
interpretierbar. Vorlagen und Vertragsentwürfe können zwischen
unabhängigen DCS-Instanzen ausgetauscht werden.

### Verhandlung und formalisierter Abschluss

Aus Vorlagen erzeugte Verträge durchlaufen eine definierte State-Machine
(Entwurf, Angebot, Verhandlung, Review, Freigabe, Signatur). Einzelne
Vertragsbedingungen werden als Change Requests verhandelt. Jede Instanz
führt dabei ihren eigenen Workflow mit eigenen Rollen und
Freigabeprozessen; über die Instanzgrenze wandert der Vertrag selbst,
nicht der interne Zustand der Gegenseite.

### eIDAS-konforme Signatur und Übergabe an Zielsysteme

Abgeschlossene Verträge werden als elektronisch signierte,
eIDAS-2.0-konforme und zugleich semantisch maschinenlesbare
Vertragsdokumente erzeugt: Das PDF/A-3-Dokument trägt die kanonische
JSON-LD-Repräsentation als eingebetteten Anhang und wird per
PAdES/JAdES signiert. Zielsysteme können vereinbarte Bedingungen
automatisiert umsetzen oder auswerten und vertragsrelevante Zustände,
Nachweise und Vertragsgegenstände an die beteiligten DCS-Instanzen
zurückmelden. Der Vertrag bleibt damit über seinen gesamten Lebenszyklus
eine maschineninterpretierbare Vereinbarung, die mit technischen Systemen
interagieren kann.

### Zero-Trust-Identitätsmodell

Die Architektur folgt einem Zero-Trust-Modell. Vertrauen entsteht durch
kryptographisch verifizierbare Identitäten, Credentials, elektronische
Signaturen, Vertrauenslisten und unabhängig nachvollziehbare Prüfprozesse,
nicht aus Netzwerkpositionen oder organisatorischer Zugehörigkeit.
Konkret:

- **Endnutzer** authentifizieren sich über Verifiable Presentations
  (OpenID4VP, SD-JWT VC), z. B. aus einer EUDI Wallet; daraus entsteht
  eine OIDC-Session über Ory Hydra.
- **Maschinelle Aufrufer** authentifizieren sich über eigene, vom DCS
  ausgestellte OAuth2-Clients. Ihre Rechte stehen in einer Registry der
  Instanz, nie in einem Token-Claim.
- **Jede Instanz** besitzt eine `did:web`-Identität, deren privates
  Schlüsselmaterial ausschließlich in einem PKCS#11-Token (HSM) liegt.
  Instanz-zu-Instanz-Aufrufe werden per Challenge-Response-Signatur und
  Zertifikatskette verifiziert. Zusätzlich konsultiert jede ein- und
  ausgehende Föderationsinteraktion einen lokalen Policy-Endpunkt
  (Trust-PDP), fail-closed.
- **Ausstellervertrauen ist zweckgebunden:** Ein Credential-Aussteller
  wird getrennt danach zugelassen, ob er Anmeldenachweise, Nachweise
  einer Partnerinstanz oder Identitätsnachweise einer natürlichen Person
  ausstellen darf, und für welche Organisationen er sprechen darf
  (ADR-31).

### Kryptographisch nachvollziehbare Auditierbarkeit

Vertragsänderungen, Verhandlungsereignisse, Signaturvorgänge,
Zustandsänderungen und weitere relevante Ereignisse werden in einem
manipulationsnachweisbaren Audit-Trail erfasst. Die Integrität wird
durch kryptographische Hash-Verkettungen und Merkle-Tree-Strukturen
sichergestellt: Jeder Audit-Eintrag referenziert seinen Vorgänger pro
Ressource, jeder Verankerungs-Batch wird durch einen Merkle-Checkpoint
committet, dessen Root einen RFC-3161-Zeitstempel erhält und in **IPFS**
verankert wird, einem verteilten, föderationsübergreifend nutzbaren
Speichersystem. Durch die Verankerung von Vertragsrepräsentationen und
Audit-Trail-Strukturen (bzw. ihrer Hashes und Merkle-Roots) entsteht ein
unabhängig überprüfbarer Integritätsnachweis: Es lässt sich belegen,
dass eine bestimmte Vertrags- oder Audit-Repräsentation zu einem
bestimmten Zeitpunkt in einer bestimmten Form existiert hat und
nachträglich nicht unbemerkt verändert wurde.

### Unabhängige Validierung: maschinenlesbar ↔ menschenlesbar

Ein zentraler Bestandteil des DCS ist die unabhängige Validierung der
semantischen gegen die menschenlesbare Vertragsrepräsentation. Der
Dokumentendienst **pdf-core** rendert deterministisch: Dieselbe
JSON-LD-Payload erzeugt byte-identischen sichtbaren Seiteninhalt. Jede
beteiligte Instanz extrahiert aus einem eingehenden signierten Dokument
die eingebettete JSON-LD, rendert sie unabhängig neu und vergleicht das
Ergebnis mit dem erhaltenen Dokument. Damit prüft jede Instanz
eigenständig, ob die maschinenlesbaren Vertragsbedingungen der für
Menschen sichtbaren Darstellung entsprechen, ohne sich auf die Aussage der
ausstellenden oder übermittelnden Instanz zu verlassen. Eine
**C2PA-Provenance-Kette** im Dokument macht zusätzlich die Herkunft jedes
sichtbaren Bytes nachweisbar.

### Vertraulichkeit und Löschbarkeit

Jedes in IPFS abgelegte Artefakt ist verschlüsselt. Der Schlüssel ist pro
Vertrag zufällig gezogen und liegt im Ruhezustand ausschließlich in einer
Form vor, die nur das HSM der jeweiligen Instanz öffnen kann. Die
Löschung eines Vertrags im Sinne von Art. 17 DSGVO ist deshalb die
protokollierte Vernichtung dieser Schlüsselkopien auf beiden beteiligten
Instanzen. Bytes aus einem Speicher zu entfernen, der konstruktionsbedingt
nicht löschen kann, wäre keine belastbare Zusage. Die Merkle-Beweise des
Audit-Trails bleiben gültig, weil sie nie vom lesbaren Inhalt abhingen
(ADR-28, Kapitel 7 und 9).

## Zusammenfassung

Das DCS verbindet föderierte digitale Identitäten, Verifiable
Credentials, semantische Vertragsmodelle, ODRL-basierte maschinenlesbare
Vertragsbedingungen, interoperable Vertragsverhandlung, eIDAS-konforme
elektronische Signaturen, automatisierte Vertragsumsetzung und
kryptographisch verankerte Auditierbarkeit in einer gemeinsamen
Infrastruktur. Es bildet damit eine föderierte Vertrauens- und
Kommunikationsinfrastruktur für digitale Verträge.

Wie diese Fähigkeiten auf Komponenten verteilt sind, beschreibt
[Kapitel 02 Architektur](02-architektur.md).
