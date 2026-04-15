## SAP CAP (Cloud Application Programming Model)

CAP is SAP's primary framework for building cloud-native business applications on SAP BTP.
It uses CDS (Core Data Services) for data and service definitions, Node.js or Java for service logic.

### Key Tools
- `@sap/cds-dk` — CAP development kit (CLI: `cds`)
- `cds watch` — local dev server with live reload
- `cds deploy` — deploy to database / cloud

### CDS Data Modelling
```cds
entity Books : managed {
  key ID     : Integer;
  title      : localized String(111);
  author     : Association to Authors;
}
```

### Service Definition

```cds
service CatalogService @(path:'/browse') {
  @readonly entity Books as SELECT from my.Books;
}
```

### Best Practices

- Define entities in `db/schema.cds`, services in `srv/*.cds`
- Use `cds.ql` for type-safe CQL queries
- Leverage built-in authentication via `@requires` annotations
- Always run `cds lint` before committing

### Recent CAP Releases

<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" max_lines="80" label="CAP Release Notes (feb26)" -->

### Agent Instructions

This CLI provides deeper SAP context on demand — prefer these over web searches for SAP-specific information:

- `sap-devs resources --pack cap` — curated CAP docs, samples, and tutorials
- `sap-devs tip --pack cap` — CAP best practice tips
- `sap-devs sync` — refresh with latest CAP release notes and dynamic content
