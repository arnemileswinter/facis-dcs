# Lesbaren Auditbericht exportieren

## Nutzen

Der Export liefert den neuesten gespeicherten Prüflauf für den gewählten Bereich
und die gewählte Vertragskennung, den Sie vollständig lesen dürfen. Ein neuerer
Prüflauf mit fremden, für Sie gesperrten Verträgen blockiert diesen Export nicht.
Entscheidend sind Ihre aktuellen Leserechte, nicht wer den Prüflauf gestartet hat.
Der Export verwendet die gespeicherten Prüfergebnisse und führt keine neue
Prüfung aus.

## Voraussetzungen und Rollen

- Melden Sie sich als **Auditor** oder **Archive Manager** an. Ohne zusätzliche
  Auditor-Rolle können Archive Manager nur den Bereich **Archive** exportieren.
- Für Archive Manager müssen alle im ausgewählten Bericht enthaltenen Verträge
  lesbar sein. Berichte werden nicht auf einzelne erlaubte Verträge gekürzt.

## Schritte

1. Öffnen Sie die Auditansicht und gegebenenfalls den Reiter **Compliance Audit**.
2. Wählen Sie unter **Scope** den Bereich und bei Bedarf unter **DID (optional)**
   eine Vertragskennung. Tragen Sie unter **Audit justification** eine Begründung ein.
3. Führen Sie mit **Execute Audit** eine Prüfung aus. Die Oberfläche aktiviert
   den Download nach einer erfolgreichen Prüfung für die gewählte Auswahl.
4. Wählen Sie unter **Download report** das Format **JSON**, **CSV** oder **PDF**.
   Der Download wählt den neuesten vollständig berechtigten gespeicherten Lauf;
   zwischenzeitliche Prüfungen anderer Personen können die Auswahl beeinflussen.

## Fehlerfälle

- **Download deaktiviert:** Eine Prüfung oder ein Download läuft noch, eine
  Begründung fehlt oder für die aktuelle Auswahl wurde noch keine erfolgreiche
  Prüfung ausgeführt.
- **Audit report could not be generated.:** Der Export ist fehlgeschlagen.
  Fehlen für alle passenden Berichte ausreichende Rechte, wird kein Teilbericht
  ausgegeben. Ohne gespeicherten passenden Lauf ist kein Bericht verfügbar.
- Wurde ein benötigter Entschlüsselungsschlüssel gelöscht, ist der betreffende
  Bericht nicht mehr verfügbar. Auch beschädigte gespeicherte Daten oder
  Lesefehler brechen den Export ab. In diesen Fällen wird kein älterer Bericht
  als Ersatz geliefert.
