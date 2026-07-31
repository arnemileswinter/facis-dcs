# Störungen und häufige Fragen

Dieses Kapitel sammelt die Meldungen und Randfälle, die Ihnen im Alltag
begegnen können, und sagt, was Sie dann tun können.

## Anmeldung und Zugriff

**Der QR-Code auf der Anmeldeseite läuft ab.**
QR-Code und Anmeldelink erneuern sich selbstständig, solange der Reiter
geöffnet bleibt (etwa alle fünf Minuten). Schließen Sie die Seite nicht,
während Sie in der Wallet arbeiten. Ist der Code doch abgelaufen, laden Sie
die Seite neu.

**Ich werde immer wieder auf die Startseite zurückgeworfen.**
Sie haben eine Ansicht aufgerufen, für die Ihnen die Rolle fehlt. Welche
Rollen Sie tragen, zeigt **User Details**. Rollen werden nicht in der
Anwendung vergeben, sondern über die Nachweise in Ihrer Wallet.

**„insufficient permissions: requires one of […]".**
Der Server hat eine einzelne Aktion abgelehnt. Die Sichtbarkeit einer
Schaltfläche allein verschafft keine Berechtigung; jede Aktion wird zusätzlich
serverseitig geprüft.

## Vorlagen

**„Global Name is required." / „Base Description is required."**
Pflichtangaben fehlen. Die Ansicht springt in das erste unvollständige Feld;
es wurde nichts gespeichert. Ergänzen Sie die Angaben und wiederholen Sie
**Create**.

**Ich finde keine Schaltfläche „Verify" in der Prüfansicht.**
Die fachliche Verifikation ist Bestandteil von **Approve** und wird bei jedem
Freigabeversuch genau einmal ausgeführt. Sie lässt sich nicht überspringen.

**Der Verifikationsdialog listet Beanstandungen und bietet keine
Bestätigung an.**
Das ist beabsichtigt: Solange die Verifikation etwas beanstandet, kann die
Vorlage nicht weitergeleitet werden. Lassen Sie die genannten Punkte
überarbeiten und starten Sie die Freigabe erneut.

**„Verification unavailable" im Dialog.**
Die Verifikation ist technisch fehlgeschlagen. **Retry** wiederholt sie. Die
Vorlage wurde nicht weitergeleitet.

**Meine freigegebene Vorlage erscheint nicht bei der Vertragsanlage.**
Freigabe (APPROVED) genügt nicht — die Vorlage muss vom Template Manager noch
**registriert** werden (Status REGISTERED / in use).

**Der Reiter „Data" sagt, es seien keine Formbibliotheken registriert.**
Semantische Datenobjekte stehen erst zur Verfügung, wenn im Semantic Hub eine
Formbibliothek veröffentlicht wurde. Das erledigt der Template Manager.

**Ein Land fehlt in einer räumlichen Bedingung.**
Die Länder- und Regionenliste stammt aus dem Fachkatalog und lässt sich nicht
als freier Text ergänzen. Wenden Sie sich an den Template Manager, der den
Semantic Hub pflegt.

## Verträge

**„Offer to counterparty" ist ausgegraut.**
Der Vertrag ist noch nicht vollständig ausgefüllt. Der Hinweis am Mauszeiger
nennt das fehlende Pflichtfeld. Ein Angebot muss vollständig sein, bevor es
das Haus verlässt.

**Nach dem Anbieten ist die Schaltfläche verschwunden.**
Ein Angebot wird genau einmal abgegeben (DRAFT → OFFERED). Weitere Änderungen
laufen ab dann über die Verhandlung.

**„Submit" in der Verhandlung ist gesperrt.**
Es sind noch Änderungsvorschläge offen. Eine Einigung setzt voraus, dass beide
Seiten alle offenen Vorschläge entschieden haben. Einen eigenen Vorschlag
können Sie nicht selbst annehmen.

**Mein gespeicherter Verhandlungsentwurf ist weg.**
**Change Proposal** verbraucht den privaten Entwurf: Was verbindlich
vorgeschlagen wurde, existiert nicht mehr als Entwurf. Ein Entwurf bleibt
dagegen erhalten, wenn Sie die Ansicht nur verlassen und später zurückkehren.

**Die semantische Vorprüfung meldet Punkte.**
Eine Bestätigung ist dann nicht verfügbar. Lassen Sie die Punkte beheben und
starten Sie **Approve** erneut. Meldet der Dialog stattdessen einen
technischen Fehler, hilft **Retry verification**; war die Vorprüfung
erfolgreich und nur die Weiterleitung schlug fehl, hilft **Retry submission**
— Ihr Kommentar bleibt erhalten.

**Der Vertrag bleibt nach der Signatur auf SIGNED stehen.**
Wahrscheinlich ist kein Zielsystem gesetzt. Die Karte **Deployment target**
weist darauf hin. Der Contract Manager kann das Ziel nachtragen und die
Inkraftsetzung mit **Deploy** anstoßen.

**„Deploy" wird abgelehnt.**
Ein Vertrag wird nur ausgerollt, wenn alle vorgesehenen Unterschriften
nachweisbar vorliegen und ein Zielsystem benannt ist.

**Ein Vertrag mit drei oder mehr Parteien wird nicht ausgerollt.**
Das ist eine bekannte Einschränkung: Der Nachweis der Unterschrift einer
Gegenseite wird derzeit je Vertrag geführt, nicht je Unterschriftsfeld.
Verträge mit mehr als zwei Parteien verweigert das System deshalb bewusst die
Ausführung, statt eine unvollständige Signaturlage als vollständig zu
behandeln. Nutzen Sie bis auf Weiteres Verträge mit zwei Parteien.

**„Terminate" lässt sich nicht abschließen.**
Der Bestätigungsdialog verlangt eine Begründung (**Reason**). Ohne Begründung
wird nichts ausgeführt.

## Signieren

**Der Vertrag steht nicht in meiner Signierliste.**
Nur freigegebene Verträge (APPROVED) erscheinen dort. Fragen Sie beim Contract
Approver nach.

**Schritt 3 oder 4 ist gesperrt.**
Die Schritte sind bewusst nacheinander freigeschaltet. Erst die erfolgreiche
Integritätsprüfung (**Verify**) gibt den Download frei, erst der Download den
Upload.

**„… does not identify the verified signatory".**
Das Zertifikat, mit dem das PDF unterschrieben wurde, lautet auf einen anderen
Namen als der in der Wallet vorgelegte Identitätsnachweis. Verwenden Sie ein
Signaturmittel, dessen Zertifikat auf Ihren eigenen Namen ausgestellt ist, und
laden Sie erneut hoch. Der Vertrag bleibt unsigniert.

**Meine Vollmacht wird abgelehnt.**
Die Zeichnungsvollmacht muss auf Sie und auf die unterzeichnende Organisation
lauten, gültig (nicht widerrufen oder ausgesetzt) sein, einen prüfbaren
Statusnachweis enthalten und auf einen für Ihre Umgebung zugelassenen
Vertrauensanker zurückführen. Fehlt einer dieser Punkte, wird der
Signaturvorgang nicht fortgesetzt. Wenden Sie sich an die ausstellende Stelle.

**Der Signaturvorgang bricht mit Fristablauf ab.**
Für jeden Signaturvorgang gilt eine Frist. Bestätigt die Wallet erst danach,
wird die Vorlage abgelehnt. Starten Sie neu über **Open & sign**.

**Mein PDF-Programm meldet „nach der Signatur verändert".**
Das ist erwartet und kein Fehler: Nach dem Signieren fügt der DCS dem Dokument
einen Herkunftsnachweis an, damit die Nachweiskette lückenlos bleibt.
Prüfprogramme werten jede Ergänzung nach der Unterschrift als Änderung. Die
Signatur selbst bleibt gültig und unverändert prüfbar.

## Prüfung, Compliance und Archiv

**Die Exportschaltflächen (JSON/CSV/PDF) sind grau.**
Ein Export setzt einen erfolgreichen Prüflauf für genau den eingestellten
Prüfumfang voraus. Ändern Sie Umfang oder Kennung, sperren sich die
Schaltflächen wieder, bis erneut geprüft wurde.

**Der Prüflauf schlägt technisch fehl.**
Es gibt bewusst keinen automatischen zweiten Versuch und keinen ersatzweise
erzeugten Befund. Starten Sie einen neuen Lauf erst, wenn die Ursache behoben
ist.

**Ein gerade geänderter Datensatz taucht in der Prüfung nicht auf.**
Die Prüfhistorie wird in einem unveränderlichen Ablagespeicher geführt;
unmittelbar nach einer Änderung kann ein einzelner Eintrag noch nicht abrufbar
sein. Wiederholen Sie die Prüfung.

**Ein Vertragsübergang bleibt hängen („manuelle Prüfung erforderlich").**
Ein Compliance Officer muss den gespeicherten Prüflauf entscheiden. Ändern Sie
den Vertrag in der Zwischenzeit nicht — sonst wird die Fortsetzung abgelehnt
und der Übergang muss für den aktuellen Stand neu angefordert werden.

**„Content erased — encryption keys destroyed" beim Export.**
Für diesen Vertrag wurde ein Löschersuchen ausgeführt; die Inhalte sind
dauerhaft nicht mehr lesbar. Die Stammdaten des Vertrags bleiben sichtbar. Ein
Wiederherstellen ist ausgeschlossen.

**Die Löschschaltfläche im Archiv bleibt inaktiv.**
Der Dialog verlangt beides: eine Begründung und das ausdrückliche Häkchen zur
Unwiderruflichkeit.

**Das Archiv-Dashboard zeigt einen gerade signierten Vertrag noch nicht.**
Der Archiveintrag entsteht kurz nach dem Signieren. Laden Sie die Ansicht nach
einem Moment erneut.

## Weitere bekannte Einschränkungen

**Prüfung des angezeigten Vertragstexts.**
Beim Signieren wird geprüft, ob der sichtbare Text dem maschinenlesbaren
Inhalt entspricht. Diese Prüfung lässt sich mit einem gezielt präparierten
PDF-Dokument umgehen. Die Behebung ist als eigener Arbeitsschritt vorgesehen.
Prüfen Sie im Zweifel den Vertragsinhalt zusätzlich über die Ansicht
**Machine-readable view**, die unmittelbar aus den gespeicherten
Vertragsdaten erzeugt wird.
