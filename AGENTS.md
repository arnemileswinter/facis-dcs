# AGENTS.md

Hinweise fuer Agenten und Automatisierung in diesem Repository.

## Grundregeln

- Das Produkt ist noch nicht released. Es muss keine Ruecksicht auf Legacy-Daten oder bestehende produktive Datenbankeintraege genommen werden.
- Hardcoding vermeiden. Konfiguration, Vokabulare, Rollen, Endpunkte und fachliche Konstanten sollen aus den bestehenden Config-, Schema-, Ontologie- oder Katalogquellen kommen.
- Bestehende Architektur und lokale Patterns verwenden, statt neue Strukturen parallel aufzubauen.
- Generierten Code nicht manuell editieren. Bei Goa-Aenderungen die Quellen in `backend/design/` anpassen und danach `goa gen digital-contracting-service/design` ausfuehren.
- Unrelated Aenderungen im Working Tree nicht zuruecksetzen. Vor allem lokale Zertifikate, `.env`-Dateien und generierte Laufzeitdateien nur anfassen, wenn die Aufgabe es verlangt.

## Requirements-Quellen

- `docs/SRS_FACIS_DCS.txt` ist die primaere, maschinenlesbare Quelle fuer Requirements, Use Cases und Akzeptanzkriterien.
- `docs/SRS_FACIS_DCS.pdf` ist die originale Referenz. Bei erkennbaren Extraktions-, Formatierungs- oder Inhaltsabweichungen in der TXT-Fassung gilt die PDF-Fassung.
- Freigegebene ADRs und finale Entscheidungsdokumente sind fuer die darin ausdruecklich entschiedenen Architektur- und Produktfragen bindend. Entwuerfe oder Dokumente mit ausstehender Entscheidung sind nicht bindend.
- Eine konkrete Nutzeranforderung oder ein Ticket ist eine zusaetzliche Scope-Quelle. Widerspricht sie der SRS oder einer freigegebenen Entscheidung, darf der Konflikt nicht eigenmaechtig aufgeloest werden.
- Die urspruengliche Use-Case-Matrix ist nur Traceability-Quelle, sofern sie fuer die Aufgabe bereitgestellt wird.
- Implementierungsstatus wird aus Code, Tests und nachvollziehbarer Verifikation ermittelt. Planungslabels oder Dokumentationsbehauptungen sind kein Erfuellungsnachweis.

## Projektueberblick

Der Digital Contracting Service (DCS) ist eine Full-Stack-Anwendung zum Erstellen, Signieren, Verwalten und Pruefen digitaler Vertraege. Die Anwendung integriert EUDI/OIDC, semantische Vertragsdaten, PDF/C2PA-Provenance, Signaturmanagement, Audit/Compliance, DCS-zu-DCS-Synchronisation und Kubernetes-basierte Entwicklungs-/Deployment-Umgebungen.

Die Hauptanwendung besteht aus einem Go-Backend und einer Vue-Frontend-App. Im Docker-/Deployment-Betrieb dient der Go-Service sowohl die API als auch die gebaute UI aus. In der lokalen Entwicklung laeuft das Frontend ueber Vite und proxyt API-Aufrufe an den Backend-Port.

## Wichtige Verzeichnisse

```text
.
├── backend/              Go-Backend, Goa-API-Design, Business-Logik, DB-Migrationen
├── frontend/ClientApp/   Vue 3/Vite/Pinia Frontend
├── pdf-core/             Separater Go-Service fuer deterministische PDF/A-3a-Erzeugung
├── deployment/           Helm-Charts, lokale Stack-Doku, Node-RED Deployment UI
├── features/             Behave/Gherkin BDD-Szenarien auf Produktebene
├── steps/                Python-Step-Implementierungen fuer die BDD-Szenarien
├── tests/bdd/            Kind-/Helm-Test-Harness fuer BDD-Integrationstests
├── testWallet/           Demo-Wallet und SD-JWT/OID4VP-Testhilfen
├── docs/                 Spezifikationen, ADRs, Ontologien, Ablaufdiagramme
│   └── SRS_FACIS_DCS.txt  Primaere, maschinenlesbare Requirements-Quelle
└── dev-stack.sh          Ein-Kommando-Start fuer lokale Helm-Abhaengigkeiten, Backend und UI
```

## Backend

Pfad: `backend/`

- Sprache/Frameworks: Go 1.25+, Goa v3, sqlx/PostgreSQL, NATS/CloudEvents, OIDC/Hydra, Keycloak/Federated Catalogue Integration.
- Entrypoints:
  - `cmd/dcs/`: HTTP-API-Server auf Port `8991`.
  - `cmd/dcs-cli/`: CLI-nahe Hilfsfunktionen.
- API-Vertrag:
  - `design/`: Goa DSL, autoritative Quelle fuer Endpunkte, Typen und Fehler.
  - `gen/`: Goa-generierter Code. Nicht direkt editieren.
- Fachlogik:
  - `internal/auth/`: Authentifizierung, OIDC/OID4VP, Audit, Token-/Rollenlogik.
  - `internal/templaterepository/`: Template Repository, Commands, Queries, DB, Events, Self-Description.
  - `internal/contractworkflowengine/`: Vertragsworkflows, Commands/Queries, Negotiation/Merging, Remote Sync.
  - `internal/signingmanagement/`: Signaturprozesse, DSS-Anbindung, DB und Events.
  - `internal/pdfgeneration/`: PDF-Erzeugung und Anbindung an `pdf-core`.
  - `internal/processauditandcompliance/`: Audit- und Compliance-Abfragen/Events.
  - `internal/templatecatalogueintegration/`: Federated Catalogue Integration.
  - `internal/dcstodcs/`: DCS-zu-DCS-Kommunikation und Synchronisation.
  - `internal/base/`: Geteilte Infrastruktur fuer Config, DB, Events, IPFS, KV, TSA, Validation.
- Datenbank:
  - `migrations/sql/`: SQL-Migrationen.
  - `migrations/fcschemas/`: Federated-Catalogue-Schema-Synchronisation und Mapping.

Wichtige Backend-Befehle:

```bash
cd backend
go mod tidy
goa gen digital-contracting-service/design
go run ./cmd/dcs
air
go test -v ./...
./bin/golangci-lint run
```

## Frontend

Pfad: `frontend/ClientApp/`

- Stack: Vue 3, Vite, TypeScript, Pinia, Vue Router, Axios, Tailwind/DaisyUI.
- `src/services/`: API-Clients. Services sollen duenn bleiben und HTTP-Aufrufe kapseln.
- `src/models/services/`: Service-Interfaces und API-nahe Typen.
- `src/stores/`: Globale Pinia-Stores fuer Auth, Session, Listen, Caches, Navigation und Fehlerzustand.
- `src/modules/template-repository/`: Feature-Modul fuer Template Repository und Editor.
- `src/models/dcs-jsonld.ts`: Zentrale Typdefinitionen fuer DCS JSON-LD.
- `src/router/router.ts`: Routing und Guards fuer Authentifizierung/Rollen.

Wichtige Frontend-Befehle:

```bash
cd frontend/ClientApp
npm install
npm run dev
npm run build
npm run lint
npm run format:check
npx vue-tsc --noEmit
```

Frontend-Konfiguration liegt in Vite-Env-Dateien. `DCS_API_PATH`, `DCS_API_TARGET` und `DCS_UI_PATH` steuern API-Pfad, Proxy-Ziel und UI-Basispfad.

## Template Editor und semantische Daten

- Der Template Editor verwendet `dcsDraftStore` als Single Source of Truth fuer das aktive JSON-LD-Dokument.
- `dcs:sections` ist eine flache, geordnete Liste aus Clauses und TextBlocks.
- ODRL/SLA-Daten werden als konkrete Typen modelliert, nicht als abstrakte Regeln.
- Fachliche Kataloge wie SLA-Actions, Metriken, Operatoren und Units kommen aus dem bestehenden Ontologie-/Katalog-Code, insbesondere `modules/template-repository/utils/sla-ontology-catalog.ts`.
- Keine hartcodierten fachlichen Listen in Komponenten einbauen.

## PDF Core

Pfad: `pdf-core/`

`pdf-core` ist ein separater Go-Service fuer deterministische PDF/A-3a-Erzeugung aus JSON-LD. Er bettet kanonische JSON-LD-Payloads ein und erzeugt C2PA-Provenance. Wichtige Endpunkte sind `/download`, `/verify`, `/update`, `/claim` und Ontologie-Endpunkte unter `/ontology/dcs-pdf-core`.

Wichtige Befehle:

```bash
cd pdf-core
go test ./...
go run .
```

## Deployment und lokale Entwicklung

Pfad: `deployment/`

- `deployment/helm/`: Parent Helm Chart plus Subcharts fuer PostgreSQL, Keycloak, Hydra, NATS und Crypto Provider.
- `deployment/helm/values.dev.yml`: lokale Entwicklungswerte.
- `deployment/helm/values.bdd.yml`: Werte fuer BDD/kind.
- `deployment/node-red/`: Node-RED-basierte Deployment-/Editor-Integration.
- `dev-stack.sh`: Startet Helm-Abhaengigkeiten, bereitet Backend-Env/Zertifikate vor und startet Backend mit `air` sowie Frontend mit Vite.

Lokaler Standardstart:

```bash
bash dev-stack.sh
```

Manuelle Entwicklung nutzt normalerweise:

```bash
helm dependency update ./deployment/helm
helm upgrade --install dcs ./deployment/helm -f ./deployment/helm/values.dev.yml
cd backend && air
cd frontend/ClientApp && npm run dev
```

## Tests

- Backend-Unit-/Integrationstests: `cd backend && go test -v ./...`
- PDF-Core-Tests: `cd pdf-core && go test ./...`
- Frontend: `npm run lint`, `npm run build`, `npx vue-tsc --noEmit`.
- Produktweite BDD-Szenarien liegen in `features/`.
- Step-Implementierungen liegen in `steps/`.
- BDD-Harness fuer kind/Helm liegt in `tests/bdd/`.

BDD-Befehle:

```bash
# Schneller Standard fuer die lokale/agentische Iteration:
make -C tests/bdd kind_up_single
make -C tests/bdd run_bdd_kind_fast_once F=features/<PATH> NEEDS_ORCE=0

# Vollstaendiger Stack beziehungsweise berichtender Lauf:
make -C tests/bdd kind_up
make -C tests/bdd run_bdd_kind_once
make -C tests/bdd run_bdd_kind_once F=features/<PATH>
make -C tests/bdd kind_down
make -C tests/bdd run_bdd_kind_ci
```

Teststrategie fuer Agenten:

- Implementierer fuehren zuerst nur die betroffenen Go-Packages, Frontend-Pruefungen
  und das direkt betroffene Feature aus.
- Fuer nicht signaturbezogene Features den Single-Instance-Stack ohne DSS und den
  Fast-Runner ohne Coverage/JUnit verwenden. `NEEDS_ORCE=0` ueberspringt zusaetzlich
  die ORCE-Teststeuerung, wenn das Feature sie nicht benoetigt.
- Den persistenten Stack zwischen fokussierten Laeufen wiederverwenden; nicht fuer
  jede Iteration Images, Cluster und Helm-Releases neu aufbauen.
- Nur der unabhaengige Verifier oder CI fuehrt den vollstaendigen BDD-Lauf aus.
  Mehrere Agenten duerfen nicht gleichzeitig Harness-Laeufe gegen denselben
  Cluster, Namespace oder dieselben lokalen Ports starten.
- Lange BDD-Laeufe mit einem Timeout oberhalb der erwarteten Suite-Dauer starten
  und ueber den laufenden Prozess warten. Harte Tool-Abbrueche vermeiden.

Die Step-Architektur ist dreischichtig:

- `steps/core/`: Authentifizierung und generische Assertions.
- `steps/support/`: HTTP-Client, URL-Building und wiederverwendbare Support-Services.
- Feature-Step-Dateien: Gherkin-Bindings, die Services aufrufen und Testzustand im Behave-Kontext halten.

## Arbeitsweise bei Aenderungen

- Backend-API-Aenderungen beginnen in `backend/design/`; danach Goa-Code generieren und Implementierung in `backend/internal/...` anpassen.
- DB-Aenderungen als neue Migration in `backend/migrations/sql/` ablegen. Da es noch keine Release-/Legacy-Daten gibt, muessen keine Rueckwaertskompatibilitaets-Pfade fuer alte Daten modelliert werden.
- Frontend-API-Aufrufe ueber Services kapseln und bestehende Stores/Module verwenden.
- Editor-/JSON-LD-Aenderungen gegen `src/models/dcs-jsonld.ts` und die bestehenden Ontologie-/Katalogquellen pruefen.
- Bei fachlichen Konstanten zuerst nach vorhandenen Configs, Mappings, Schemas, Ontologien oder Values suchen.
- Tests in dem Umfang ergaenzen oder anpassen, der dem Risiko der Aenderung entspricht.

## Pre-Commit und generierte Artefakte

Im Repository-Root installiert `npm install` Husky. Die Hooks fuehren lint-staged fuer Frontend-Dateien sowie Go-Linting und `go mod tidy`-Checks fuer Backend-Dateien aus.

Generierte oder lokale Artefakte nur bewusst veraendern:

- `backend/gen/`: Goa-generiert, nie manuell editieren.
- `frontend/ClientApp/dist/`: Build-Ausgabe.
- `backend/.env`, `backend/certs/dev/chain.pem`: lokale Laufzeitkonfiguration bzw. Dev-Zertifikatskette.
- `node_modules/`: Abhaengigkeiten, nicht manuell editieren.
