# Vorlagen verwalten (Template Manager)

Als Template Manager machen Sie freigegebene Vorlagen produktiv nutzbar: Sie
registrieren sie im Vorlagenkatalog, behalten den Katalog im Blick und pflegen
über den Semantic Hub den zentralen Fachwortschatz, auf dem alle Vorlagen
aufbauen.

**Voraussetzungen:** Sie sind mit der Rolle Template Manager angemeldet. In
der Seitennavigation sehen Sie **Templates**, **Template Catalogue** und
**Semantic Hub**.

## Eine freigegebene Vorlage registrieren

Öffnen Sie die Vorlage über **Templates** → **View**.

![Vorlage registrieren](images/template-manager-registrieren.png)

Die Fortschrittsleiste steht auf **Approved**. Die Überschrift darunter lautet
„This template is approved", und der Erläuterungstext nennt den verbleibenden
Schritt: Ein Template Manager registriert die Vorlage im Template Catalogue,
und nur registrierte Vorlagen lassen sich für Verträge verwenden. Der erste
Abschnitt der Vorlage **(1)** trägt rechts das Statusabzeichen **APPROVED**.

Mit **Register** **(2)** veröffentlichen Sie die Vorlage im Vorlagenkatalog;
ein Bestätigungsdialog fragt vor der Ausführung nach („Proceed with
registration?"), den Sie mit **Confirm** abschließen. Daneben stehen **Copy**
(die Vorlage als neuen Entwurf kopieren), **Archive** (die Vorlage aus dem
aktiven Bestand nehmen), **Export PDF** und **Back**. Als Template Manager
sehen Sie zusätzlich den Reiter **Audit History** mit der protokollierten
Geschichte der Vorlage.

Nach der Registrierung trägt die Vorlage den Status REGISTERED (in use) und
kann von Contract Creators als Grundlage neuer Verträge gewählt werden. Erst
ab diesem Zeitpunkt erscheint sie in deren Vorlagenauswahl.

**Register** steht nur bei Vorlagen vom Typ Contract zur Verfügung. Eine
Komponente wird nicht registriert; sie gelangt über die Vertragsvorlage in
Gebrauch, die sie einbettet.

An einer registrierten Vorlage tritt die Schaltfläche **Publish** hinzu. Sie
stellt die Vorlage in den Verbundkatalog, sodass andere Organisationen sie
finden können; auch hier bestätigen Sie mit **Confirm**. Die Vorlage trägt
danach den Status PUBLISHED und bleibt weiterhin für Verträge verwendbar.

## Eine Vorlage stilllegen (Archive)

**Archive** nimmt eine Vorlage aus dem aktiven Bestand; der
Bestätigungsdialog fragt „Proceed with archiving?" und wird mit **Confirm**
abgeschlossen. Die Wirkung hängt vom Stand der Vorlage ab:

- Eine **registrierte** (oder bereits veröffentlichte) Vorlage erhält den
  Status **DEPRECATED**. Aus ihr lassen sich keine neuen Verträge mehr
  ableiten; sie verschwindet aus der Vorlagenauswahl der Contract Creators.
  Bestehende Verträge laufen unverändert weiter und tragen in der
  Vertragsübersicht das Abzeichen „This contract uses a deprecated template".
- Eine noch nicht registrierte Vorlage erhält den Status **DELETED** und ist
  damit aus dem aktiven Bestand entfernt.

Bei einer bereits stillgelegten Vorlage (DELETED oder DEPRECATED) wird
**Archive** nicht mehr angeboten; ein erneutes Stilllegen ist nicht möglich.

## Eine registrierte Vorlage aktualisieren

Eine registrierte Vorlage wird nicht direkt geändert. Legen Sie über **Copy**
eine Kopie als neuen Entwurf an; sie durchläuft Prüfung und Freigabe und wird
anschließend als nächste Version derselben Vorlage registriert. Ältere
Fassungen bleiben über die Historie erhalten und bereits erzeugte Verträge
unverändert gültig.

Sobald die neue Version registriert ist, trägt die Vorgängerfassung in der
Vorlagenübersicht das Abzeichen „A newer template version is available" als
Verweis auf die neue Fassung. Dasselbe Abzeichen erscheint in der
Vertragsübersicht an Verträgen, die noch auf der Vorgängerfassung beruhen.

## Vorlagenkatalog

Der Bereich **Template Catalogue** listet die im Verbundkatalog
veröffentlichten Vorlagen.

![Vorlagenkatalog](images/template-manager-katalog.png)

Über das Suchfeld **(1)** finden Sie eine Vorlage; der Umschalter davor
stellt zwischen der Suche nach **DID** und nach **Name** um, **Sort by**
ändert die Sortierung. Jeder Eintrag **(2)** nennt Name, Version, Kennung und
Beschreibung. Die letzte Zeile eines Eintrags sagt, woher die Vorlage stammt:
**In catalogue** bedeutet, dass sie im Verbundkatalog liegt, aber nicht in
Ihrem eigenen Bestand; **In local repository** bedeutet, dass sie Ihnen
bereits vorliegt.

**View** **(3)** öffnet die Detailansicht mit den Reitern **Details**, **Meta
Data** und **Preview**. Unter **Preview** lesen Sie das Dokument der
Katalogfassung, bevor Sie sie übernehmen. **Register** übernimmt eine fremde
Vorlage in Ihren eigenen Bestand; nach der Bestätigung mit **Confirm** legt
der DCS daraus eine lokale Vorlage an, die eine eigene Kennung erhält.

Ist in Ihrer Installation kein Verbundkatalog eingerichtet, bleibt der Bereich
leer, und Veröffentlichen wie Katalogsuche stehen nicht zur Verfügung. Ob und
wie ein Katalog betrieben wird, entscheidet der Betreiber Ihrer Installation.
Weist der Katalog den Abruf ab, erscheint statt der Liste eine Fehlermeldung;
wenden Sie sich dann an den Betrieb.

## Semantic Hub: Fachwortschatz pflegen

Der Semantic Hub ist das zentrale Verzeichnis der Fachbegriffe, Regelwerke und
Prüfschemata, aus denen der Klausel-Editor seine Auswahllisten bezieht: von
den Länderlisten räumlicher Bedingungen bis zu den Formbibliotheken für
semantische Datenobjekte. Neue Einträge lassen sich im laufenden Betrieb
veröffentlichen, ohne Softwareänderung.

![Semantic Hub](images/template-manager-semantic-hub.png)

Links unter **Entries** listet die Ansicht die registrierten Einträge mit
ihrer Art (**context**, **ontology**, **shapes**, **profile**) und ihrem
Versionsstand, etwa „active v1" und „latest v1"; ohne freigeschaltete Version
steht dort „no active version". Solange kein Eintrag gewählt ist, steht
rechts „Select an entry to inspect its versions."

Ein Klick auf einen Eintrag zeigt rechts seine Versionen als Tabelle mit
**Version**, **Status** (das Abzeichen **active** an der wirksamen Version),
**Registered by** und **Registered at**. Je Version stehen zwei Aktionen
bereit:

- **View** blendet den gespeicherten Inhalt der Version ein.
- **Activate** macht eine andere Version zur wirksamen. So kehren Sie auch zu
  einer früheren Version zurück: Neue Dokumente entstehen dann wieder gegen
  diesen Stand, während bereits erzeugte Dokumente an die Version gebunden
  bleiben, unter der sie entstanden sind.

Unter der Tabelle registriert **Register new version** eine weitere Version
desselben Eintrags; das Formular entspricht dem Veröffentlichen-Formular
unten, mit dem Kontrollkästchen **Activate immediately**.

<!-- Screenshot fehlt: Versionstabelle eines Semantic-Hub-Eintrags mit den Aktionen View und Activate. Grund: in diesem Lauf wurden keine Screenshots erzeugt. -->

Welche Regelfassung bei einem Vertrag geprüft wird, richtet sich nicht nach
der gerade aktiven Version: Jeder Vertrag ist an den Regelstand gebunden, der
bei seiner Erzeugung aktiv war, und wird auch später genau dagegen geprüft.
Eine strengere neue Fassung wirkt daher nur auf danach erzeugte Dokumente.

Unterhalb der Liste veröffentlichen Sie unter **Publish new entry** neue
Einträge:

1. Vergeben Sie unter **Name** einen eindeutigen Namen.
2. Wählen Sie unter **Kind** die Art des Eintrags (z. B. **shapes** für eine
   Formbibliothek, aus der Template Creators Datenobjekte zusammenklicken).
3. Bringen Sie den Inhalt ein. Der Reiter **Paste / upload** nimmt ihn im
   Feld **Content** direkt entgegen; über **Upload file** daneben laden Sie
   stattdessen eine Datei. Der Reiter **From URL** holt den Inhalt unter
   **Source URL** von einer Adresse und legt ihn als Version ab.
4. Unter **Version (optional)** tragen Sie eine feste Versionsnummer ein,
   wenn Sie eine anderswo veröffentlichte Bibliothek genau in der Version
   installieren, auf die deren Vorlagen verweisen. Ohne Angabe entsteht die
   nächste Version.
5. **Activate immediately** legt fest, ob die neue Version sofort aktiv wird.
6. **Publish entry** veröffentlicht den Eintrag.

![Veröffentlichter Eintrag im Semantic Hub](images/template-manager-semantic-hub-veroeffentlicht.png)

Der neue Eintrag **(1)** erscheint anschließend in der Liste mit dem Vermerk
**active** und steht ab sofort systemweit zur Verfügung. Im Beispiel ist es
eine Formbibliothek, aus der Template Creators im Reiter **Data**
Datenobjekte zusammenklicken können. Eine bereits veröffentlichte Version
bleibt erhalten; ein erneutes Veröffentlichen legt eine weitere Version an.
Bereits erzeugte Dokumente bleiben an die Version gebunden, unter der sie
entstanden sind.
