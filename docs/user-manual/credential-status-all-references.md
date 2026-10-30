# Berechtigungsnachweis einer Signatur prüfen

## Nutzen

Der Signature Compliance Viewer zeigt den aktuellen Status des bei der Signatur
vorgelegten Vollmachtsnachweises. Bei mehreren Statusangaben reicht eine einzelne
gültige Angabe nicht aus, um den Nachweis als gültig anzuzeigen.

## Voraussetzungen und Rollen

Sie benötigen einen angemeldeten, berechtigten Zugang zum Compliance Viewer
sowie einen dort sichtbaren signierten Vertrag.

## Schritte

1. Öffnen Sie den **Signature Compliance Viewer** und wählen Sie den Vertrag aus.
2. Wechseln Sie zu **Compliance Checks**.
3. Lesen Sie unter **Signature level detail** für die betreffende Signatur die
   Spalte **Credential status** und eine gegebenenfalls darunter angezeigte
   Fehlerbegründung.

## Status und Fehlerfälle

- **VALID:** Alle geprüften Statusangaben sind gültig und die Prüfung akzeptiert
  den Nachweis. Dies ist keine Aussage über sämtliche anderen Signaturprüfungen.
- **REVOKED:** Der ausgewertete Status weist den Nachweis als widerrufen aus.
- **SUSPENDED:** Der ausgewertete Status weist den Nachweis als ausgesetzt aus.
- **UNKNOWN:** Der Status ist nicht sicher bestimmbar, beispielsweise weil eine
  Statusliste nicht erreichbar ist, der Nachweis fehlt oder die Vertrauensprüfung
  scheitert. Beachten Sie die angezeigte Begründung; UNKNOWN bestätigt keine
  Gültigkeit.

Enthält der Nachweis mehrere unterschiedliche nicht gültige Statusangaben, wird
die erste davon angezeigt. Eine davor stehende gültige Angabe verdeckt sie
nicht.
