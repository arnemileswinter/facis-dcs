---
name: ui-verbesserung
description: Analysiert, implementiert und verifiziert FACIS-DCS-UI-/UX-Verbesserungen einschließlich sichtbarer Full-Stack-Fehler. Verwenden bei Vue-Komponenten, Formularen, Admin-Oberflächen, Editor-Zuständen, responsiven Ansichten, Accessibility und browserbasierten UI-Regressionsprüfungen; bei geändertem Produktverhalten zusammen mit bdd-anforderung verwenden.
---

# FACIS-DCS-UI verbessern

## Verbindliche Grundlagen

1. Lies `AGENTS.md` sowie relevante SRS-, ADR- und Entscheidungsfundstellen.
2. Verwende den bestehenden Vue-/Pinia-/Service-Aufbau, Tailwind/DaisyUI und
   vorhandene UI-Komponenten. Führe kein paralleles Designsystem ein.
3. Beziehe fachliche Werte aus bestehenden Config-, Schema-, Ontologie- und
   Katalogquellen. Hardcode keine Rollen, Zustände oder Vokabulare in
   Komponenten.
4. Nutze für neues oder geändertes Produktverhalten zusätzlich
   `bdd-anforderung`; dieser Skill ersetzt dessen Requirements-, Architektur-
   und Verifier-Gates nicht.

## Rollen und Reihenfolge

Führe schreibende Rollen wegen des gemeinsamen Working Trees sequenziell aus.
Übergib nur ein kompaktes Handoff mit Anforderung, ACs, Quellen, betroffenen
Dateien, beobachtetem Ist-Zustand und Testplan.

1. `ui-analyst`: Reproduziert beziehungsweise lokalisiert den sichtbaren
   Fehler lesend. Bewertet Nutzerfluss, Informationshierarchie, Form-Semantik,
   Tastaturbedienung, Zustände und responsive Breakpoints. Liefert konkrete
   UI-Akzeptanzkriterien und verweist auf bestehende Komponenten/Patterns.
2. Nutze für fachliche oder komponentenübergreifende Änderungen die Gates aus
   `bdd-anforderung`. Kleine, vollständig freigegebene Darstellungsregressionen
   dürfen dessen COMPACT-Pfad verwenden.
3. `ui-implementierer`: Ändert den kleinsten zusammenhängenden Produktpfad.
   Frontend-Services bleiben dünn; Stores und zentrale JSON-LD-Typen bleiben
   autoritativ. Backend-API-Änderungen beginnen in `backend/design/` und werden
   mit Goa generiert.
4. `ui-verifier`: Prüft unabhängig jedes UI-AC gegen Code und tatsächlich
   ausführbare Tests. Verifiziert mindestens Disabled-/Read-only-Semantik,
   Tastaturpfad, Lade-/Leer-/Fehlerzustände und relevante Breakpoints.

## UI-Qualitätsregeln

- Verwende `disabled` für nicht ausführbare Aktionen und `readonly` nur für
  weiterhin fokussierbare, kopierbare Werte. Visuelle Gestaltung allein genügt
  nicht.
- Gruppiere lange Formulare nach Aufgabe und Abhängigkeit. Nutze klare
  Abschnittsüberschriften, Labels, Hilfetexte, konsistente Abstände und eine
  erkennbare primäre Aktion.
- Erhalte bei Vergleichsansichten Inhalt, Scrollbarkeit und Orientierung auf
  breiten Bildschirmen; auf schmalen Viewports muss eine eindeutige gestapelte
  oder umschaltbare Darstellung bestehen.
- Decke Erfolg, Leerzustand, Laden, Validierungsfehler, Serverfehler und
  fehlende Berechtigung ab, soweit der betroffene Flow diese Zustände besitzt.
- Vermeide rein kosmetische Großumbauten außerhalb des betroffenen Flows.

## Zeiteffiziente Verifikation

1. Prüfe zuerst gezielte Unit-/Component-Tests und betroffene TypeScript-Dateien.
2. Führe danach `npm run lint`, `npx vue-tsc --noEmit` und einen Build mit
   Ausgabe unter `/tmp` aus, soweit der Scope sie erfordert.
3. Verwende einen bereits laufenden Stack für Browserprüfungen. Starte oder
   mutiere keinen Cluster, wenn ein anderer Job ihn nutzt.
4. Nutze für nicht signaturbezogene BDD-Iteration den persistenten
   Single-DCS-Stack und
   `run_bdd_kind_fast_once F=features/<PATH> NEEDS_DSS=0 NEEDS_ORCE=0`.
5. Überlasse den berichtenden vollständigen BDD-Lauf dem unabhängigen
   Verifier; führe niemals parallele Harness-Läufe gegen denselben Cluster,
   Namespace oder dieselben Ports aus.

Berichte je AC Implementierung, automatisierte Nachweise, Browser-/Responsive-
Nachweise und verbleibende Einschränkungen.
