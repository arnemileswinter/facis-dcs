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

Die Fortschrittsleiste steht auf **Approved**, der Hinweis darunter nennt den
verbleibenden Schritt: „Approved — one step left: a Template Manager registers
it in the Template Catalogue; only registered templates can be used to create
contracts." Der erste Abschnitt der Vorlage **(1)** trägt rechts das
Statusabzeichen **APPROVED**.

Mit **Register** **(2)** veröffentlichen Sie die Vorlage im Vorlagenkatalog;
ein Bestätigungsdialog fragt vor der Ausführung nach („Proceed with
registration?" — bestätigen Sie mit **Confirm**). Daneben stehen **Copy** (die
Vorlage als neuen Entwurf kopieren), **Archive** (die Vorlage aus dem aktiven
Bestand nehmen), **Export PDF** und **Back**. Als Template Manager sehen Sie
zusätzlich den Reiter **Audit History** mit der protokollierten Geschichte der
Vorlage.

Nach der Registrierung trägt die Vorlage den Status REGISTERED (in use) und
kann von Contract Creators als Grundlage neuer Verträge gewählt werden. Erst
ab diesem Zeitpunkt erscheint sie in deren Vorlagenauswahl.

## Vorlagenkatalog

Der Bereich **Template Catalogue** listet die im Verbundkatalog
veröffentlichten Vorlagen.

Über die Suche finden Sie eine Vorlage nach Name oder Kennung (DID); die
Detailansicht eines Katalogeintrags zeigt die veröffentlichten Metadaten.

![Vorlagenkatalog](images/template-manager-katalog.png)

Der Vorlagenkatalog mit veröffentlichten Vorlagen: Über das Suchfeld **(1)**
finden Sie eine Vorlage; der Umschalter davor stellt zwischen der Suche nach
**DID** und nach Namen um, **Sort by** ändert die Sortierung. Jeder Eintrag
**(2)** nennt Name, Version, Kennung und Beschreibung. Die letzte Zeile eines
Eintrags sagt, woher die Vorlage stammt: **In catalogue** — die Vorlage liegt
im Verbundkatalog, aber nicht in Ihrem eigenen Bestand; **In local
repository** — sie liegt Ihnen bereits vor. **View** **(3)** öffnet die
Detailansicht mit den veröffentlichten Metadaten.

Ist in Ihrer Installation kein Verbundkatalog eingerichtet, bleibt der Bereich
leer — Veröffentlichen und Katalogsuche stehen dann nicht zur Verfügung. Ob
und wie ein Katalog betrieben wird, entscheidet der Betreiber Ihrer
Installation. Weist der Katalog den Abruf ab, erscheint statt der Liste eine
Fehlermeldung; wenden Sie sich dann an den Betrieb.

## Semantic Hub: Fachwortschatz pflegen

Der Semantic Hub ist das zentrale Verzeichnis der Fachbegriffe, Regelwerke und
Prüfschemata, aus denen der Klausel-Editor seine Auswahllisten bezieht — von
den Länderlisten räumlicher Bedingungen bis zu den Formbibliotheken für
semantische Datenobjekte. Neue Einträge lassen sich im laufenden Betrieb
veröffentlichen, ohne Softwareänderung.

![Semantic Hub](images/template-manager-semantic-hub.png)

Links unter **Entries** listet die Ansicht die registrierten Einträge mit
ihrer Art (**context**, **ontology**, **shapes**, **profile**) und dem
Versionsstand (**active** / **latest**); ein Klick auf einen Eintrag zeigt
rechts seine Versionen („Select an entry to inspect its versions.").

Unterhalb der Liste veröffentlichen Sie neue Einträge:

1. Vergeben Sie unter **Name** einen eindeutigen Namen.
2. Wählen Sie unter **Kind** die Art des Eintrags (z. B. **shapes** für eine
   Formbibliothek, aus der Template Creators Datenobjekte zusammenklicken).
3. Fügen Sie unter **Content** den Inhalt ein — wahlweise per Direkteingabe,
   Dateiupload (**Upload file**) oder Abruf **From URL**. Mit **Activate
   immediately** legen Sie fest, ob die neue Version sofort aktiv wird.
4. **Publish entry** veröffentlicht den Eintrag.

![Veröffentlichter Eintrag im Semantic Hub](images/template-manager-semantic-hub-veroeffentlicht.png)

Der neue Eintrag **(1)** erscheint anschließend in der Liste mit dem Vermerk
**active** und steht ab sofort systemweit zur Verfügung — im Beispiel eine
Formbibliothek, aus der Template Creators im Reiter **Data** Datenobjekte
zusammenklicken können. Eine bereits veröffentlichte Version bleibt erhalten;
ein erneutes Veröffentlichen legt eine weitere Version an.
