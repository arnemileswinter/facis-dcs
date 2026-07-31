# Archiv verwalten und Verträge abrufen (Archive Manager)

Sobald ein Vertrag signiert ist, wird er automatisch archiviert — mit einer
Momentaufnahme des Dokuments, den Signaturangaben, einer Prüfsumme des Inhalts
und einem Zeitstempelnachweis. Als Archive Manager überblicken Sie diesen
Bestand, rufen archivierte Verträge samt ihrer Nachweiskette ab und führen
Löschersuchen aus.

**Voraussetzungen:** Sie sind mit der Rolle Archive Manager angemeldet. In der
Seitennavigation sehen Sie **Archive** und **Audit**. Auditoren sehen
dieselben Bereiche (siehe [Audits durchführen](auditor.md)).

## Das Archiv-Dashboard

Öffnen Sie **Archive** in der Seitennavigation.

![Archiv-Dashboard](images/archive-dashboard.png)

Die Ansicht **Contract Archive Dashboard** gliedert sich in vier Bereiche:

1. Die **Statistikleiste** **(1)** — **Archived contracts** (Zahl der
   archivierten Verträge), **Storage volume** (belegtes Speichervolumen),
   **Compliant** (Einträge mit vollständigen Nachweisen), **Flagged**
   (Einträge mit unvollständigen Nachweisen) und **Deleted** (Einträge, deren
   Inhalt gelöscht wurde).
2. **Archived contracts** — die Liste der Archiveinträge mit dem
   Löschstatus je Eintrag. Liegt kein Eintrag vor, sagt die Liste das
   ausdrücklich.
3. **Expiring within 30 days** **(2)** — die Verträge, deren Gültigkeit
   demnächst endet. Welcher Zeitraum als „demnächst" gilt, ist systemseitig
   einstellbar; Verträge außerhalb des Fensters werden nicht als auslaufend
   geführt.
4. **Recent actions** **(3)** — die jüngsten Archivvorgänge, jeweils mit dem
   betroffenen Vertrag als Verweis.

Die Abbildung zeigt das Dashboard einer Installation ohne Archivbestand — so
sehen Sie die leeren Zustände, die die Ansicht ausdrücklich benennt, statt
eine leere Tabelle zu zeigen.

![Statistikleiste des Archivs](images/archive-statistiken.png)

Die Statistikleiste im Detail.

<!-- Screenshot fehlt: Archiv-Dashboard mit Einträgen (Liste, Löschstatus-Abzeichen, letzte Aktionen) — Grund: auf der bereitgestellten Instanz lässt sich kein Signaturvorgang abschließen — das Hochladen des signierten Dokuments wird von der automatischen Regelprüfung abgewiesen —, daher wird dort nichts archiviert; das Archiv ist leer. Der Ablauf ist durch die End-to-End-Tests belegt. -->

## Einen archivierten Vertrag abrufen

Ein Archiveintrag wird über den Audit-Arbeitsplatz abgerufen — dort erhalten
Sie nicht nur den Vertrag, sondern seine vollständige, prüfbare
Archivhistorie.

1. Klicken Sie im Bereich **Recent actions** auf den gewünschten Vertrag.
   Die Anwendung wechselt in den Audit-Arbeitsplatz und stellt dort bereits
   alles ein: **Scope** steht auf **archive**, das Feld **DID** enthält die
   Kennung des Vertrags.
2. Tragen Sie unter **Audit justification** die Begründung Ihres Abrufs ein.
3. **Execute Audit** ruft die Archivhistorie des Vertrags ab.

<!-- Screenshot fehlt: Archivhistorie eines Vertrags im Audit-Arbeitsplatz — Grund: der Prüfdienst der bereitgestellten Instanz liefert inzwischen Ergebnisse, es existiert dort aber kein einziger Archiveintrag, dessen Historie abgerufen werden könnte (es wird nichts archiviert, siehe oben). Der Ablauf ist durch die automatisierten Abnahmetests belegt. -->

Das Ergebnis zeigt unter **Checks** die Nachweise des Archiveintrags und unter
**Timeline** die Ereignisse des Vertrags in zeitlicher Folge — von der
Erstellung über Signatur und Inkraftsetzung bis zu späteren Änderungen. Über
**JSON**, **CSV** und **PDF** exportieren Sie den Nachweis; der Export
verlangt eine eigene Begründung.

Kennen Sie die Kennung des Vertrags bereits, können Sie den Audit-Arbeitsplatz
auch direkt öffnen, **Scope** auf **Archive** stellen und die Kennung
eintragen — der Weg über das Dashboard erspart Ihnen nur das Abtippen.

Das Archiv lässt sich darüber hinaus gezielt durchsuchen: nach einer
beteiligten Vertragspartei (gefunden werden genau die Verträge, an denen diese
Partei beteiligt ist) sowie nach dem Gültigkeitszeitraum (gefunden werden die
Verträge, deren Gültigkeit vollständig im angefragten Zeitraum liegt). Ein
Suchlauf mit einem nicht lesbaren Datum wird mit einer Fehlermeldung
abgewiesen.

## Der Löschstatus eines Vertrags

Jeder Archiveintrag trägt ein Abzeichen, das den Zustand seiner
Verschlüsselung anzeigt:

- **Keys live** — die Inhalte des Vertrags sind abrufbar.
- **Keys destroyed** — die Schlüssel wurden vernichtet; die Inhalte sind
  dauerhaft nicht mehr lesbar.

Denselben Zustand zeigt der Audit-Arbeitsplatz im Abschnitt zur Löschung an;
unter **Erasure details** listet er zusätzlich auf, welche
Partnerorganisationen die Löschung bestätigt haben.

## Einen Vertragsinhalt löschen

Ein Löschersuchen (z. B. nach Art. 17 DSGVO) wird umgesetzt, indem die
Schlüssel des Vertragsinhalts vernichtet werden. Das ist **endgültig**: Der
Inhalt kann danach von niemandem mehr entschlüsselt werden — auch nicht vom
Betreiber.

1. Wählen Sie am Archiveintrag die Löschaktion.
2. Der Bestätigungsdialog benennt die Folge ausdrücklich: „Encryption keys
   will be destroyed on both instances (irreversible)".

<!-- Screenshot fehlt: Bestätigungsdialog der Archivlöschung mit Begründungsfeld, Unwiderruflichkeits-Häkchen und Bestätigungsschaltfläche — Grund: auf der bereitgestellten Instanz existiert kein Archiveintrag, an dem sich die Löschaktion aufrufen ließe, weil dort kein Signaturvorgang abgeschlossen werden kann. Der Ablauf ist durch die End-to-End-Tests belegt. -->

3. Tragen Sie im Textfeld die Begründung des Löschersuchens ein.
4. Setzen Sie das Bestätigungshäkchen. Erst dann wird die
   Bestätigungsschaltfläche aktiv — ohne Begründung und ohne ausdrückliche
   Bestätigung führt der Dialog nichts aus.
5. Nach der Ausführung meldet die Ansicht „Archive entry deleted — encryption
   keys destroyed"; der Eintrag verschwindet aus der Liste.

Bei einem Vertrag mit einer Partnerorganisation werden die Schlüssel auf
beiden Seiten vernichtet. Der Audit-Arbeitsplatz zeigt anschließend den
Zustand **Keys destroyed** und unter **Erasure details** die Bestätigung der
Gegenseite.

### Was nach der Löschung bleibt

- Der Vertrag bleibt in der Vertragsübersicht mit seinen Stammdaten (Name,
  Version, Status) sichtbar — gelöscht werden die Inhalte, nicht der
  Verwaltungseintrag.
- Ein Versuch, das Vertragsdokument zu exportieren, endet mit der klaren
  Meldung „Content erased — encryption keys destroyed". Die Ansicht bleibt
  bedienbar.
- Die Löschung selbst ist ein protokolliertes Ereignis und im
  Audit-Arbeitsplatz nachweisbar.
