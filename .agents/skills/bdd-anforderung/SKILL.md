---
name: bdd-anforderung
description: Fuehrt eine konkrete oder mutmasslich gemeinte, aber noch unvollstaendige FACIS-DCS-Produktanforderung beziehungsweise ein Ticket risikobasiert durch einen FULL- oder COMPACT-BDD-Loop mit projektspezifischen Subagents. Verwenden, wenn Produktverhalten geklaert, analysiert, spezifiziert, implementiert und unabhaengig verifiziert werden soll; nicht fuer reine Dokumentations-, Infrastruktur-, CI- oder Formatierungsaenderungen ohne Produktverhalten.
---

# BDD-Anforderung umsetzen

## Verbindliche Quellen laden

1. Lies `AGENTS.md`.
2. Lies vor jeder fachlichen oder technischen Entscheidung die relevanten
   Abschnitte aus `docs/SRS_FACIS_DCS.txt`. Nutze
   `docs/SRS_FACIS_DCS.pdf`, um Extraktions-, Formatierungs- oder
   Inhaltsabweichungen der TXT-Fassung zu klären.
3. Lies thematisch relevante freigegebene ADRs und finale
   Entscheidungsdokumente. Entwürfe und ausstehende Entscheidungen sind
   nicht bindend. Bei einem Konflikt zwischen SRS, freigegebener Entscheidung
   und Nutzeranforderung melde `STATUS: NEEDS-INPUT` und löse ihn nicht
   eigenmächtig auf.
4. Falls die Nutzerangabe einen existierenden Dateipfad enthält, lies ihn als
   zusätzliche Anforderungsquelle.

## Orchestrierung

Führe ein kompaktes Handoff-Paket mit Anforderung, Requirement-Slug, AC-Tabelle,
exakten Quellenfundstellen, Architekturentscheidung, Abhängigkeitsentscheidung,
betroffenen Dateien, Testplan und offenen Punkten. Übergib jeder Rolle nur
dieses Paket und benötigte Vorphasenergebnisse. Starte Fachagenten mit
`fork_turns="none"`; vollständige Thread-Historie ist weder Übergabe noch
Ersatz für das Handoff. Die Rollen lesen referenzierte Quellen selbst und
erweitern die Suche nur bei Lücken oder Widersprüchen. Warte auf abhängige
Phasen. Schreibende Phasen laufen wegen des gemeinsamen Working Trees
sequenziell.

1. `analyst`: Leite Requirement-Slug, atomare ACs mit stabilen IDs,
   Prüfmittel (`BDD`, `extern-validiert`, `grep-gate`, `manueller-Drill`),
   Traceability-IDs und eine begründete Workflow-Empfehlung ab. Bei `STATUS:
   NEEDS-INPUT` halte an und frage den Nutzer. Lass auch einen bereits
   erkennbaren Quellenkonflikt vom `analyst` ausschließlich lesend mit den
   kollidierenden Fundstellen formalisieren; starte danach keine weitere Rolle.
2. Wähle `FULL`, wenn mindestens eines gilt: neues oder geändertes
   freigegebenes Sollverhalten; neue oder geänderte API, Persistenz,
   Migration, Security-, Identitäts-, Signatur-, Provenance-, Föderations-
   oder Protokolllogik; mehrere Komponenten; neue Abhängigkeit;
   Architekturentscheidung; fehlende oder zu ändernde BDD-Abdeckung;
   fachliche Unsicherheit. Die reine Wiederherstellung bereits freigegebenen
   Sollverhaltens ist keine Sollverhaltensänderung. Nutzerwunsch `FULL` ist
   bindend.
3. Wähle `COMPACT` nur, wenn alles gilt: lokal begrenzte, risikoarme
   Regression; Verhalten und ACs sind vollständig freigegeben; keine der
   FULL-Bedingungen trifft zu; ehrliche bestehende Akzeptanzabdeckung muss
   nicht geändert werden. Im Zweifel oder bei späterer Abweichung eskaliere
   nach `FULL`.
4. Führe einen frühen, ausschließlich lesenden Testumgebungs-Preflight für die
   benötigten Prüfmittel aus. Prüfe Tools, Konfiguration und Erreichbarkeit;
   starte oder ändere keine Infrastruktur. Halte das Ergebnis im Handoff fest.
   Bestimme FULL oder COMPACT unabhängig von der Umgebung. Ist ein zwingendes
   Prüfmittel nicht ausführbar, keine gleichwertige alternative Evidenz
   vorhanden und die Bereitstellung nicht vom Nutzerauftrag umfasst, halte vor
   jeder Schreibphase mit `STATUS: BLOCKED`; implementiere nicht auf Vorrat.

## FULL

1. `architekt`: Erstelle bei einer echten Architekturentscheidung ein
   ADR-lite, andernfalls eine Feasibility-Notiz. Bewerte jede AC-ID mit `GO`,
   `NEEDS-CLARIFICATION` oder `NO-GO`. Halte bei `NO-GO`; beginne bei
   inkonsistentem Klärungsstand keine Schreibphase. Bewerte Libraries nur,
   wenn ein neuer technischer Baustein oder eine Abhängigkeit zur Entscheidung
   steht.
2. Setze vor der Schreibphase `DISCOVERY-GATE: SATISFIED`, wenn eine
   freigegebene Anforderung bereits konkrete Beispiele und Entscheidungen
   enthält. Als Freigabenachweis gelten ausdrücklich bestätigte Angaben des
   Nutzers beziehungsweise Tickets oder Beispiele aus SRS und finalen
   Entscheidungen; ein Planungslabel allein genügt nicht. Fordere andernfalls
   bei neuem oder geändertem Fachverhalten eine menschliche Freigabe der ACs
   und Beispiele an. Frage nicht erneut nach bereits ausdrücklich entschiedenen
   Punkten.
3. `gherkin-autor`: Implementiere GO-ACs mit Prüfmittel `BDD` als rote
   Spezifikation unter `features/` und `steps/`. Nutze Requirement-Tags sowie
   bei Föderation `@two-instance` und `BDD_DCS_BASE_URL_A/_B`. Verwende gegen
   einen vorhandenen kind-Stack den kleinsten Lauf, typischerweise
   `run_bdd_kind_fast_once F=features/<PATH>` mit passenden
   `NEEDS_DSS`-/`NEEDS_ORCE`-Schaltern. Nutze `run_bdd_fast` nur gegen eine
   bereits anderweitig erreichbare Umgebung. Dokumentiere `RED-PROVEN` oder
   bei erst nach dem Preflight ausgefallener Umgebung ehrlich `RED-NOT-RUN`.
   Halte bei `RED-NOT-RUN` vor dem `implementierer`, solange kein
   gleichwertiger Red-Nachweis möglich oder die Umgebung wieder erreichbar ist.
4. Prüfe den Spezifikations-Diff und übergib dem `implementierer` die von ihm
   nicht zu ändernden Dateien.
5. `implementierer`: Ändere Produktivcode und gezielte Unit-Tests, nicht
   `features/`, `steps/` oder `tests/bdd/`. Goa-Änderungen beginnen in
   `backend/design/`, danach folgt die Generierung. Nutze fokussierte Tests,
   die betroffene Suite und den kleinsten passenden BDD-Lauf; überlasse die
   vollständige Abnahme dem `verifier`.
6. `verifier`: Führe die erforderlichen Abnahmetests unabhängig erneut aus und
   prüfe jedes GO-AC gemäß Prüfmittel. Nutze vorhandene Infrastruktur; starte
   oder ändere sie nicht. Ein nicht ausführbarer Lauf ist `STATUS: BLOCKED`.
   Bei roten Tests oder fehlender Abdeckung gehe für betroffene ACs zu Schritt
   3 oder 5 zurück.
7. `revisor`: Starte erst nach vollständig grüner Verifikation. Ändere nur bei
   belegtem Nutzen. Rufe nach jeder materiellen Quellenänderung erneut den
   `verifier` auf; nur dessen grünes Ergebnis schließt die Revision ab.
8. `dokumentierer`: Starte nach abgeschlossener Revision. Schreibe nur unter
   `docs/` und nur soweit Requirement, Nutzerwirkung und Code dies belegen.
   `NO-DOC-CHANGE` ist für rein interne Änderungen zulässig.

## COMPACT

1. Belege vorhandene, unverändert ausreichende Akzeptanzabdeckung im Handoff.
   Fehlt sie oder muss sie geändert werden, eskaliere nach `FULL`. Setze
   `DISCOVERY-GATE: NOT-APPLICABLE`, weil COMPACT ausschließlich bereits
   freigegebenes Sollverhalten wiederherstellt.
2. `implementierer`: Implementiere nur die freigegebenen ACs und gezielte
   Tests. Nutze den kleinsten passenden Testlauf. Ändere niemals
   `features/`, `steps/` oder `tests/bdd/`; jede dort erforderliche Änderung
   eskaliert nach `FULL`.
3. `verifier`: Wiederhole die betroffenen Tests unabhängig und prüfe jedes AC.
   Bei fehlender Abdeckung, Architekturwirkung oder Scope-Ausweitung
   eskaliere nach `FULL`.

Wiederhole Spezifikation, Implementierung und Verifikation bis alle ACs
erfüllt sind. Beende den Loop nicht mit laufenden Subagents. Fasse abschließend
Modus, Discovery-Gate, AC-Status und Prüfmittel, Testnachweise,
Architekturentscheidung, Revision, Dokumentation und offene Punkte zusammen.

## Sicherheitsregeln

- Setze keine unabhängigen Working-Tree-Änderungen zurück.
- Bearbeite `backend/gen/` nie manuell.
- Führe ohne ausdrücklichen Nutzerauftrag weder `git commit` noch `git push`
  aus.
- Behandle Rollen-Sandboxen als zusätzliche Leitplanken, nicht als Ersatz für
  die Pfad- und Scope-Regeln in den Agentenprofilen.
- Starte keinen vollständigen BDD-Stack und keinen vollständigen BDD-Lauf
  mehrfach parallel gegen denselben Cluster, Namespace oder dieselben Ports.

## Wartung

Lies nur beim Ändern oder Evaluieren dieses Skills
`references/evaluation-cases.md` und führe danach
`python3 scripts/validate.py` aus. Lade die Evaluationsfälle nicht in normale
Anforderungsläufe.
