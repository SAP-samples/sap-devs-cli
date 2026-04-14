## cds watch für lokale Entwicklung nutzen
Tags: cap,nodejs
`cds watch` statt `node server.js` ausführen — es lädt bei jeder Dateiänderung neu und protokolliert alle Anfragen.

## managed-Entities für Audit-Felder definieren
Tags: cap,cds
`: managed` zu Entities hinzufügen, um `createdAt`, `createdBy`, `modifiedAt`, `modifiedBy` automatisch zu erhalten.

## @readonly in der Service-Schicht verwenden
Tags: cap,odata,security
`@readonly` in der Service-Schicht statt auf DB-Ebene einschränken — hält das Schema flexibel.

## CAP-Versionskompatibilität prüfen
Tags: cap,versions
`cds version` ausführen, um alle CAP-Stack-Versionen anzuzeigen. Nicht übereinstimmende `@sap/cds`- und `@sap/cds-dk`-Versionen verursachen schwer zu findende Fehler.
