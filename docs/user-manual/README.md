# Benutzerhandbuch: Digital Contracting Service (DCS)

## Was der DCS ist

Der Digital Contracting Service (DCS) begleitet Verträge über ihren gesamten
Lebenszyklus: von wiederverwendbaren Vertragsvorlagen über die
Vertragserstellung und die Verhandlung mit einer Partnerorganisation bis zu
Signatur, Inkraftsetzung, Überwachung, Archivierung und Prüfung.

Ein Vertrag im DCS hat immer zwei Seiten. Das **lesbare Vertragsdokument**
unterschreiben Menschen. Den **maschinenlesbaren Vertragsinhalt** kann das
System auswerten, prüfen und an ein Zielsystem übergeben. Beide entstehen aus
derselben Quelle und können deshalb nicht auseinanderlaufen. Jeder Schritt
(Anlegen, Ändern, Anbieten, Freigeben, Signieren) wird revisionssicher
festgehalten und bleibt später nachvollziehbar.

Die Anmeldung erfolgt ohne Benutzername und Passwort: Sie weisen sich mit
Ihrer digitalen Wallet aus. Aus den dort vorgelegten Nachweisen ergeben sich
Ihre Rollen und damit alles, was Sie im System sehen und tun dürfen.

## Aufbau des Handbuchs

Jede Aufgabe im DCS ist einer Rolle zugeordnet; dieses Handbuch ist deshalb
nach Rollen gegliedert. Lesen Sie das Kapitel Ihrer Rolle. Die Querverweise
führen Sie zu den angrenzenden Schritten.

| Kapitel | Rolle | Inhalt |
| --- | --- | --- |
| [Anmeldung und Navigation](anmeldung-und-navigation.md) | alle | Anmeldung mit der Wallet, Startseite, Seitennavigation, Zugriffsschutz |
| [Vorlagen erstellen](template-creator.md) | Template Creator | Komponenten und Vertragsvorlagen entwerfen, Klauseln mit maschinenlesbaren Regeln |
| [Vorlagen prüfen](template-reviewer.md) | Template Reviewer | Eingereichte Vorlagen prüfen und zur Freigabe weiterleiten |
| [Vorlagen freigeben](template-approver.md) | Template Approver | Geprüfte Vorlagen endgültig freigeben oder ablehnen |
| [Vorlagen verwalten](template-manager.md) | Template Manager | Vorlagen registrieren, Vorlagenkatalog, Semantic Hub |
| [Verträge erstellen und verhandeln](contract-creator.md) | Contract Creator | Verträge aus Vorlagen ableiten, ausfüllen, anbieten, verhandeln |
| [Verträge prüfen](contract-reviewer.md) | Contract Reviewer | Verhandelte Verträge prüfen und weiterleiten |
| [Verträge freigeben](contract-approver.md) | Contract Approver | Geprüfte Verträge zur Signatur freigeben |
| [Verträge unterzeichnen](contract-signer.md) | Contract Signer | Geführte Signatur im Secure Contract Viewer mit der eigenen Wallet |
| [Verträge verwalten](contract-manager.md) | Contract Manager | Vertragsübersicht, Zielsystem, Inkraftsetzung, Signaturprüfung, Beendigung |
| [Audits durchführen](auditor.md) | Auditor | Begründete, protokollierte Prüfungen über Verträge, Vorlagen, Signaturen und Archiv |
| [Archiv verwalten und Verträge abrufen](archive-manager.md) | Archive Manager | Archivübersicht, archivierte Verträge abrufen, Löschung von Vertragsinhalten |
| [Compliance überwachen](compliance-officer.md) | Compliance Officer | Risiko-Überwachung, Vorfallmeldung, zurückgestellte Prüfungen entscheiden |
| [Systemverwaltung](systemadministrator.md) | Sys. Administrator | Zielsysteme, Systembenutzer und deren Zugangsdaten, Schlüsselverzeichnis |
| [Störungen und häufige Fragen](troubleshooting-faq.md) | alle | Fehlermeldungen, Randfälle und bekannte Einschränkungen |

## Benutzerrollen

Welche Bereiche eine Person sieht und bedienen darf, ergibt sich ausschließlich
aus den Rollen, die ihre Wallet-Nachweise ausweisen. Eine Rollenverwaltung in
der Anwendung gibt es nicht.

**Rund um Vorlagen**

- **Template Creator:** entwirft wiederverwendbare Komponenten und
  Vertragsvorlagen und reicht sie zur Prüfung ein.
- **Template Reviewer:** prüft eingereichte Vorlagen und leitet sie zur
  Freigabe weiter oder weist sie zurück.
- **Template Approver:** gibt geprüfte Vorlagen endgültig frei.
- **Template Manager:** registriert freigegebene Vorlagen im Vorlagenkatalog
  und pflegt den Fachwortschatz im Semantic Hub.

**Rund um Verträge**

- **Contract Creator:** leitet Verträge aus registrierten Vorlagen ab, füllt
  sie aus, bietet sie der Gegenseite an und verhandelt.
- **Contract Negotiator:** verhandelt bestehende Verträge; sieht dieselben
  Bereiche wie der Contract Creator, ohne die Vertragsanlage.
- **Contract Reviewer:** prüft verhandelte Verträge vor der Freigabe.
- **Contract Approver:** gibt geprüfte Verträge zur Signatur frei.
- **Contract Signer:** unterzeichnet freigegebene Verträge mit der eigenen
  Wallet und dem eigenen Signaturschlüssel.
- **Contract Manager:** führt den Vertragsbestand, legt das Zielsystem fest,
  setzt Verträge in Kraft, prüft Signaturen und beendet Verträge.
- **Contract Observer:** liest Verträge und Signaturen mit, ohne selbst
  Aktionen auszulösen.

**Organisationsweit**

- **Auditor:** führt begründete, protokollierte Prüfungen über Vorlagen,
  Verträge, Signaturen und Archivvorgänge aus und exportiert Prüfberichte.
- **Archive Manager:** überblickt das Vertragsarchiv, ruft archivierte
  Verträge ab und führt Löschersuchen aus.
- **Compliance Officer:** überwacht das System auf Regelverstöße, meldet
  Vorfälle und entscheidet zurückgestellte Prüfungen.
- **Sys. Administrator:** registriert Zielsysteme, verwaltet Systembenutzer
  und deren Zugangsdaten und sieht das Schlüsselverzeichnis ein.
- **Integration Manager:** verwaltet die Zielsysteme für die Anbindung
  angrenzender Systeme.

Eine Person kann mehrere Rollen tragen; die Navigation zeigt dann die Summe
der zugehörigen Bereiche.

## Der Vertragslebenszyklus in Kürze

Vorlagen und Verträge durchlaufen feste Stationen, die in den Ansichten als
Fortschrittsleiste erscheinen:

- **Vorlagen:** Draft → Submitted → Reviewed → Approved → Registered (in use)
- **Verträge:** Draft → Submitted → Reviewed → Approved → Signed → Active

Ein Vertrag kann daneben weitere Zustände annehmen, die die Fortschrittsleiste
nicht als eigene Station führt: **OFFERED** (der Gegenseite angeboten),
**NEGOTIATION** (in Verhandlung), **REJECTED** und **WITHDRAWN** (abgelehnt
bzw. zurückgezogen) sowie nach der Signatur **REVOKED**, **TERMINATED** und
**EXPIRED**. In den Listen erscheinen sie als Statusabzeichen.

Ein Statuswechsel geschieht ausschließlich über die Aktionen der jeweils
zuständigen Rolle (**Submit**, **Approve**, **Register**, **Offer to
counterparty**, **Deploy**, **Terminate**). Ein Statusfeld zum Bearbeiten gibt
es nicht.

## Die fünf Kernabläufe auf einen Blick

| Aufgabe | Rolle | Kapitel |
| --- | --- | --- |
| Vertrag **erstellen** | Contract Creator | [Verträge erstellen und verhandeln](contract-creator.md#einen-neuen-vertrag-anlegen) |
| Vertrag **prüfen** | Contract Reviewer | [Verträge prüfen](contract-reviewer.md#einen-vertrag-prüfen-und-weiterleiten) |
| Vertrag **freigeben** | Contract Approver | [Verträge freigeben](contract-approver.md#einen-vertrag-freigeben-oder-ablehnen) |
| Vertrag **signieren** | Contract Signer | [Verträge unterzeichnen](contract-signer.md#signieren-schritt-für-schritt) |
| Vertrag **aus dem Archiv abrufen** | Archive Manager, Auditor | [Archiv verwalten und Verträge abrufen](archive-manager.md#einen-archivierten-vertrag-abrufen) |
