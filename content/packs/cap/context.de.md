## SAP CAP (Cloud Application Programming Model)

CAP ist SAPs primäres Framework für Cloud-native Business-Anwendungen auf SAP BTP.
Es verwendet CDS (Core Data Services) für Daten- und Servicedefinitionen sowie Node.js oder Java für die Service-Logik.

### Wichtige Tools
- `@sap/cds-dk` — CAP Development Kit (CLI: `cds`)
- `cds watch` — lokaler Entwicklungsserver mit Live-Reload
- `cds deploy` — Deployment in Datenbank / Cloud

### CDS-Datenmodellierung
```cds
entity Books : managed {
  key ID     : Integer;
  title      : localized String(111);
  author     : Association to Authors;
}
```

### Service-Definition
```cds
service CatalogService @(path:'/browse') {
  @readonly entity Books as SELECT from my.Books;
}
```

### Best Practices
- Entities in `db/schema.cds` definieren, Services in `srv/*.cds`
- `cds.ql` für typsichere CQL-Abfragen verwenden
- Eingebaute Authentifizierung via `@requires`-Annotationen nutzen
