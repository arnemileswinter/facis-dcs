# Audits durchführen (Auditor)

Als Auditor führen Sie nachvollziehbare Prüfungen über den Datenbestand des
DCS aus — über Vorlagen, Verträge, Signaturen oder Archivvorgänge. Jede
Prüfung erfordert eine Begründung und wird ihrerseits protokolliert.

**Voraussetzungen:** Sie sind mit der Rolle Auditor angemeldet. In der
Seitennavigation sehen Sie **Audit**, **Archive** und **Compliance Viewer**.
Für eine auf ein einzelnes Objekt begrenzte Prüfung benötigen Sie dessen
Kennung (DID).

Ein **Archive Manager** sieht dieselben Bereiche, kann im Audit-Arbeitsplatz
aber ausschließlich den Bereich **Archive** prüfen (siehe
[Archiv verwalten und Verträge abrufen](archive-manager.md)).

## Der Audit-Arbeitsplatz

Öffnen Sie **Audit** in der Seitennavigation.

![Audit-Formular](images/auditor-audit-formular.png)

1. Wählen Sie unter **Scope** **(1)** den Prüfumfang — Vorlagen, Verträge,
   Signaturen oder Archiv.
2. Tragen Sie unter **DID (optional)** **(2)** die Kennung eines einzelnen
   Objekts ein, um die Prüfung gezielt darauf zu beschränken — ohne Angabe
   wird der gesamte Bestand des gewählten Umfangs geprüft.
3. Geben Sie unter **Audit justification** **(3)** die Begründung der Prüfung
   an; sie wird mit dem Audit gespeichert.
4. **Execute Audit** **(4)** führt die Prüfung aus.

Die Prüfung wird genau einmal an den eingerichteten Prüfdienst übergeben. Bei
einem technischen Fehler erfolgt kein automatischer Wiederholungsversuch, und
es werden keine ersatzweise erzeugten Feststellungen angezeigt — starten Sie
einen neuen Lauf erst, wenn die Ursache behoben ist.

Bei Signaturprüfungen werden nur die für die Nachweisführung erforderlichen
Angaben übergeben (Status, Zeitpunkte, Prüfsummen, Referenzen). Die
Signaturdaten selbst verlassen den DCS dabei nicht.

## Das Prüfergebnis lesen

![Ausgeführter Prüflauf](images/auditor-audit-ergebnis.png)

Ein ausgeführter Prüflauf über den Umfang **Contracts**: Die Zählleiste
**(1)** über dem Formular fasst den Lauf zusammen — **Failed Checks**,
**Passed Checks** und **Needs Review**. Darunter listet **Findings** **(2)**
die einzelnen Feststellungen; die hier gewählte Zeile ist hervorgehoben und
ihr Prüfnachweis steht rechts unter **Finding Details** **(3)**. Die
Exportschaltflächen **JSON**, **CSV** und **PDF** sind jetzt freigeschaltet,
weil für diesen Umfang ein Lauf vorliegt.

Nach dem Lauf fasst ein Banner das Ergebnis zusammen (z. B. „Audit passed. No
failed checks or review findings were returned."). Darunter:

- **Checks** listet die ausgeführten Einzelprüfungen mit Status
  (**Passed**/**Failed**), betroffener Kennung und Details — etwa den Nachweis,
  dass eine maschinenlesbare Vertragsregel zur Laufzeit durchgesetzt wird.
  Jede Feststellung nennt die angewendete Regel, das Ergebnis und den Grund.
  Auch eine erfolgreiche Prüfung darf eine leere Feststellungsliste liefern.
- **Timeline** zeigt die protokollierten Ereignisse des geprüften Objekts in
  zeitlicher Reihenfolge.
- Über die **Table filters** (Status, Finding, Component, DID) grenzen Sie die
  Ergebnistabelle ein; ein Klick auf eine Zeile öffnet rechts unter **Finding
  Details** den zugehörigen Prüfnachweis. Solange keine Zeile gewählt ist,
  steht dort „Select a row to inspect the corresponding audit evidence."

Die Prüfhistorie wird in einem unveränderlichen Ablagespeicher geführt.
Unmittelbar nach einer frischen Änderung kann ein einzelner Eintrag noch nicht
abrufbar sein; führen Sie die Prüfung dann erneut aus.

## Prüfbericht exportieren

Die Schaltflächen **JSON**, **CSV** und **PDF** exportieren den Prüfbericht.
Sie sind gesperrt, bis für den aktuell eingestellten Prüfumfang (und ggf. das
eingetragene Objekt) ein erfolgreicher Lauf vorliegt — ändern Sie Umfang oder
Kennung, sperren sie sich wieder.

1. Führen Sie zuerst eine erfolgreiche Prüfung für den gewünschten Bereich
   aus.
2. Wählen Sie **JSON**, **CSV** oder **PDF**.
3. Geben Sie die Begründung für den Export an und laden Sie den Bericht
   herunter.

Der Bericht entsteht ausschließlich aus dem zuletzt gespeicherten Prüflauf;
der Prüfdienst wird beim Export nicht erneut aufgerufen. JSON enthält das
unveränderte gespeicherte Prüfergebnis, CSV und PDF stellen denselben Lauf in
anderer Form dar. Der exportierte Inhalt wird mit seiner Prüfsumme im
Prüfprotokoll nachgewiesen; lässt sich der Bericht nicht nachweisbar ablegen,
meldet die Anwendung einen Fehler, statt einen ungesicherten Bericht
auszugeben.

## Prüfungen bei Vertragsübergängen

Beim Einreichen, Anbieten, Freigeben, Signieren und Bereitstellen eines
Vertrags wird der zum Vertrag gespeicherte Regelstand automatisch geprüft. Ein
eigener Prüflauf ist dafür nicht nötig. Drei Ausgänge sind möglich:

- **Bestanden** — der angeforderte Übergang wird fortgesetzt.
- **Manuelle Prüfung erforderlich** — der Vertrag bleibt im bisherigen
  Zustand. Ein Compliance Officer prüft den gespeicherten Lauf und genehmigt
  oder verwirft ihn mit einer Begründung (siehe
  [Compliance überwachen](compliance-officer.md)). Nach einer Genehmigung wird
  genau der zurückgestellte Übergang fortgesetzt.
- **Blockiert** — der Vertrag bleibt im bisherigen Zustand. Ein neuer Versuch
  ist erst sinnvoll, nachdem der angezeigte Regelverstoß oder technische
  Fehler behoben wurde.

Solange eine manuelle Prüfung offen ist, darf der Vertrag weder inhaltlich
noch im Zustand geändert werden — sonst wird die Fortsetzung abgelehnt, damit
eine Entscheidung nicht auf einen inzwischen geänderten Vertrag angewendet
wird.

## Signaturprüfungen im Compliance Viewer

Als Auditor haben Sie zusätzlich Zugriff auf den **Compliance Viewer** (siehe
[Verträge verwalten](contract-manager.md)). Für Sie ist dort vor allem der
Reiter **Audit Reports** relevant: **Load Audit Report** lädt die
Prüfhistorie der Signaturen eines Vertrags als Tabelle.

Je Eintrag sehen Sie die beteiligte Komponente, das Ereignis (z. B. das
Anbringen oder die Prüfung einer Signatur) und den Zeitpunkt. Trägt der
gewählte Vertrag keine Signatur, bleibt **Load Audit Report** gesperrt.

<!-- Screenshot fehlt: Reiter Audit Reports mit geladener Prüfhistorie — Grund: auf der bereitgestellten Instanz existiert kein signierter Vertrag (das Hochladen des signierten Dokuments wird dort von der automatischen Regelprüfung abgewiesen), die Schaltfläche bleibt deshalb gesperrt. Zusätzlich nimmt die Dokumentenablage der Instanz keine Einträge mehr an, sodass dort auch keine Prüfhistorie entsteht. Der Ablauf ist durch die End-to-End-Tests belegt. -->

## Sichtbare Fehlerfälle

- **Nicht berechtigt** — melden Sie sich mit der Rolle Auditor an. Ein
  Archive Manager darf im Audit-Arbeitsplatz nur den Archivbereich verwenden.
- **Ungültiger Bereich oder fehlende Begründung** — korrigieren Sie die
  Auswahl bzw. ergänzen Sie eine Begründung; die Prüfung wurde noch nicht
  übergeben.
- **Prüfdienst nicht erreichbar oder Zeitüberschreitung** — der Lauf ist
  technisch fehlgeschlagen. Die Meldung erscheint als roter Hinweis über dem
  Ergebnisbereich; die Exportschaltflächen bleiben gesperrt.
- **Ungültige Antwort des Prüfdienstes** — der Lauf wird abgelehnt und nicht
  als Ergebnis gespeichert.

  ![Abgewiesener Prüflauf](images/auditor-audit-fehler.png)

  So sieht das aus: Der Prüfdienst hat geantwortet, seine Antwort passte aber
  nicht zum Prüfauftrag. Die Anwendung zeigt den Grund als roten Hinweis,
  stellt kein Ergebnis dar und lässt **JSON**, **CSV** und **PDF** gesperrt —
  es gibt bewusst keinen ersatzweise erzeugten Befund.
- **Kein passender gespeicherter Prüflauf** — für Bereich und Objekt gibt es
  noch keinen erfolgreichen Lauf. Führen Sie zunächst die Prüfung aus.
- **Regelstand nicht auflösbar** — eine für diesen Vertrag gespeicherte Regel-
  oder Profilversion fehlt oder ist nicht erreichbar. Der Übergang bleibt
  blockiert; es wird nicht auf einen anderen Regelstand ausgewichen.
