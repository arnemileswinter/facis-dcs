# DCS Technisches Handbuch

Dieses Handbuch erklärt den Digital Contracting Service (DCS) auf
Systemebene: Aufbau, Abläufe, Schnittstellen und Betriebsverhalten.
Zielgruppe sind Entwickler, Betreiber und Integratoren, die das System
verstehen, betreiben oder anbinden wollen. Quelle der Wahrheit ist der
Code.

## Inhaltsverzeichnis

| Kapitel | Inhalt |
| --- | --- |
| [01 Systemüberblick](01-systemueberblick.md) | Was das DCS ist, Einordnung in Gaia-X/XFSC/FACIS, Kernfähigkeiten |
| [02 Architektur](02-architektur.md) | Komponentenlandkarte, Kommunikationsbeziehungen, Gesamtdiagramm |
| [03 Vorlagen und Vertragsinhalte](03-vorlagen-und-vertragsinhalte.md) | Template-Lifecycle, Semantic Hub (JSON-LD/SHACL/ODRL), Klausel-Katalog |
| [04 Vertragslebenszyklus](04-vertragslebenszyklus.md) | State-Machine, Rollen/Tasks/RBAC, Verhandlung, lokal vs. grenzüberschreitend |
| [05 Identität und Authentifizierung](05-identitaet-und-authentifizierung.md) | Endnutzer (OID4VP/SD-JWT), Maschinen-Identitäten (Hydra), Instanz (did:web + HSM), Issuer-Vertrauen |
| [06 Signaturen](06-signaturen.md) | Wallet-Ceremony, AES/QES, PAdES/JAdES, DSS, TSA, PID |
| [07 Dokumente und Provenance](07-dokumente-und-provenance.md) | pdf-core, deterministisches Rendering, C2PA, IPFS, Verschlüsselung at rest |
| [08 Föderation](08-foederation.md) | Instanz-zu-Instanz-Austausch, Trust-Modell, Federated Catalogue |
| [09 Audit und Compliance](09-audit-und-compliance.md) | Audit-Trail, Merkle-Checkpoints, Workflow-Gates, Monitoring, ODRL/OPA, Reports |
| [10 Betrieb und Monitoring](10-betrieb-und-monitoring.md) | Betriebsverhalten im Normal- und Fehlerfall: Probes, Hintergrundprozesse, Signale, Incidents |
| [Anhang A Schnittstellenreferenz](anhang-a-schnittstellenreferenz.md) | API-Gruppen, Events, well-known-Endpunkte |
| [Anhang B Entscheidungsarchiv](anhang-b-entscheidungsarchiv.md) | Verweisliste auf die Architecture Decision Records (ADRs) |

## Abgrenzung zu den übrigen Dokumenten

Dieses Handbuch erklärt. Es enthält bewusst keine Befehle, keine
Wertetabellen und keine Schritt-für-Schritt-Prozeduren.

| Frage | Dokument |
| --- | --- |
| Wie ist das System gebaut, wie läuft ein Vorgang durch, was bedeutet ein Signal? | Dieses Handbuch |
| Wie installiere, konfiguriere, aktualisiere und deinstalliere ich eine Instanz? | [`docs/deployment-guide.md`](../deployment-guide.md) |
| Wie sichere und stelle ich Daten wieder her? | [`docs/backup-integration-guide.md`](../backup-integration-guide.md) |
| Wie bediene ich das System als Fachanwender? | [`docs/user-manual/`](../user-manual/) |
| Warum wurde etwas so entschieden? | Die ADRs unter `docs/`, siehe [Anhang B](anhang-b-entscheidungsarchiv.md) |

Konkrete Werte, Befehle und Konfigurationsreferenzen stehen an genau einer
Stelle, nämlich im Deployment-Leitfaden. Wo dieses Handbuch einen solchen
Wert bräuchte, verweist es dorthin.

## Leseempfehlung

- **Neu im Projekt:** Kapitel 01 und 02, danach Kapitel 04 mit dem
  zentralen Ablauf des Systems.
- **Betreiber:** Kapitel 02, dann Kapitel 10; Kapitel 09 für Audit-Trail
  und Incident-Sicht. Ausführbare Verfahren stehen im
  Deployment-Leitfaden.
- **Integrator (Zielsysteme, Peer-Instanzen):** Kapitel 02, 07, 08 und
  Anhang A.
- **Sicherheits-/Compliance-Review:** Kapitel 05, 06, 09 und Anhang B.
