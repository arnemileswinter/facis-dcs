# 01 Systemüberblick

## Was das DCS ist

Der **Digital Contracting Service (DCS)** ist ein föderiertes,
cross-federation-fähiges und on-premise hostbares System für die sichere
B2B-Kommunikation, Verhandlung und automatisierte Verarbeitung digitaler
Verträge. Der zentrale Gedanke: **Ein digitaler Vertrag ist nicht nur ein
signiertes Dokument, sondern eine interoperable, maschineninterpretierbare
und kryptographisch nachweisbare Vereinbarung zwischen Organisationen.**

Jede Organisation betreibt ihre eigene DCS-Instanz. Instanzen
unterschiedlicher Betreiber tauschen Vorlagen und Verträge aus, ohne
einander implizit zu vertrauen. Vertrauen wird pro Interaktion
kryptographisch hergestellt und geprüft.

## Einordnung: Gaia-X, XFSC, FACIS

Das DCS entsteht im Rahmen von **FACIS** (Federation Architecture for
Composed Infrastructure Services) im Eclipse-XFSC-Ökosystem und fügt sich
in die Gaia-X-Infrastrukturlandschaft ein:

- **Federated Catalogue:** Freigegebene Vorlagen werden als
  Self-Descriptions registriert und damit föderationsweit auffindbar.
- **ORCE (XFSC Orchestration Engine):** Node-RED-basierte
  Orchestrierungsschicht für Zeitstempel, Archiv-Notariat,
  Zielsystem-Anbindung, Trust-PDP und die auslagerbaren Prüf- und
  Freigabeschritte des Compliance-Subsystems.
- **XFSC Status List Service:** Widerrufsstatus der vom DCS ausgestellten
  Lifecycle-Credentials.
- **EUDI Wallet / eIDAS 2.0:** Endnutzer authentifizieren sich über
  Verifiable Credentials (OpenID4VP); Signaturen folgen den
  eIDAS-Formaten PAdES/JAdES.

Verbindliche Anforderungsbasis ist die SRS des FACIS-DCS; die
Positionierung gegenüber den XFSC-Komponenten beschreibt ADR-5.

## Kernfähigkeiten

**Semantische Vorlagen, maschinenlesbare Bedingungen.** Organisationen
definieren auf Basis frei wählbarer Ontologien maschinenlesbare
Vertragsvorlagen. Ein instanzlokaler Semantic Hub versioniert die
JSON-LD-Kontexte, SHACL-Shapes und Validierungsprofile, gegen die jedes
Dokument erzeugt und geprüft wird, und stellt den Klausel-Katalog bereit.
Vertragsbedingungen sind mit ODRL 2.0 formal beschrieben und damit für
Menschen und Maschinen interpretierbar.

**Verhandlung und formalisierter Abschluss.** Aus Vorlagen erzeugte
Verträge durchlaufen eine definierte State-Machine von Entwurf über
Angebot, Verhandlung, Review und Freigabe bis zur Signatur. Einzelne
Bedingungen werden als Change Requests verhandelt. Jede Instanz führt
ihren eigenen Workflow mit eigenen Rollen; über die Instanzgrenze wandert
der Vertrag selbst, nicht der interne Zustand der Gegenseite.

**eIDAS-konforme Signatur und Übergabe an Zielsysteme.** Abgeschlossene
Verträge sind elektronisch signierte, eIDAS-2.0-konforme und zugleich
maschinenlesbare Dokumente: Das PDF/A-3 trägt die kanonische JSON-LD als
eingebetteten Anhang und wird per PAdES/JAdES signiert. Zielsysteme
setzen vereinbarte Bedingungen automatisiert um oder werten sie aus und
melden Zustände, Nachweise und KPI-Werte an die beteiligten Instanzen
zurück. Der Vertrag bleibt so über seinen gesamten Lebenszyklus eine
maschineninterpretierbare Vereinbarung.

**Zero-Trust-Identitätsmodell.** Vertrauen entsteht aus kryptographisch
verifizierbaren Identitäten, Credentials, Signaturen, Vertrauenslisten
und unabhängig nachvollziehbaren Prüfprozessen, nicht aus
Netzwerkposition oder organisatorischer Zugehörigkeit. Endnutzer melden
sich per Verifiable Presentation an (OpenID4VP, SD-JWT VC, z. B. aus
einer EUDI Wallet); maschinelle Aufrufer erhalten eigene OAuth2-Clients,
deren Rechte in einer Registry der Instanz stehen, nie in Token-Claims;
jede Instanz besitzt eine `did:web`-Identität, deren private Schlüssel
ausschließlich in einem PKCS#11-Token (HSM) liegen. Ausstellervertrauen
ist zweckgebunden und organisationsgebunden (ADR-31), und jede
Föderationsinteraktion konsultiert fail-closed einen lokalen
Policy-Endpunkt.

**Kryptographisch nachvollziehbare Auditierbarkeit.** Vertrags-,
Verhandlungs-, Signatur- und Zustandsereignisse landen in einem
manipulationsnachweisbaren Audit-Trail: hash-verkettete Einträge je
Ressource, gebündelt zu Merkle-Checkpoints, deren Root einen
RFC-3161-Zeitstempel erhält und in IPFS verankert wird. So lässt sich
unabhängig belegen, dass eine Vertrags- oder Audit-Repräsentation zu
einem bestimmten Zeitpunkt in einer bestimmten Form existiert hat und
nachträglich nicht unbemerkt verändert wurde.

**Unabhängige Validierung: maschinenlesbar gegen menschenlesbar.** Der
Dokumentendienst pdf-core rendert deterministisch: Dieselbe
JSON-LD-Payload erzeugt byte-identischen Seiteninhalt. Jede beteiligte
Instanz extrahiert aus einem eingehenden signierten Dokument die
eingebettete JSON-LD, rendert sie unabhängig neu und vergleicht das
Ergebnis mit dem erhaltenen Dokument. Keine Instanz ist dafür auf die
Aussage der ausstellenden oder übermittelnden Instanz angewiesen. Eine
C2PA-Provenance-Kette im Dokument macht zusätzlich die Herkunft des
sichtbaren Inhalts nachweisbar.

**Vertraulichkeit und Löschbarkeit.** Jedes in IPFS abgelegte Artefakt
ist verschlüsselt. Der Schlüssel ist pro Vertrag zufällig gezogen und im
Ruhezustand nur durch das HSM der jeweiligen Instanz zu öffnen. Die
Löschung eines Vertrags im Sinne von Art. 17 DSGVO ist die protokollierte
Vernichtung dieser Schlüsselkopien auf beiden beteiligten Instanzen; die
Merkle-Beweise des Audit-Trails bleiben gültig, weil sie nie vom lesbaren
Inhalt abhingen (ADR-28, Kapitel 07 und 09).

Das DCS verbindet damit föderierte Identitäten, Verifiable Credentials,
semantische Vertragsmodelle, ODRL-Bedingungen, interoperable
Verhandlung, eIDAS-Signaturen, automatisierte Umsetzung und
kryptographisch verankerte Auditierbarkeit in einer gemeinsamen
Vertrauens- und Kommunikationsinfrastruktur. Wie diese Fähigkeiten auf
Komponenten verteilt sind, zeigt [Kapitel 02](02-architektur.md).
