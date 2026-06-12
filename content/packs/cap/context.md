## SAP CAP (Cloud Application Programming Model)

### Overview

CAP is SAP's primary framework for building cloud-native business applications on SAP BTP.
It uses CDS (Core Data Services) for data and service definitions, Node.js or Java for service logic.

### Key Concepts
- `@sap/cds-dk` — CAP development kit (CLI: `cds`)
- `cds watch` — local dev server with live reload
- `cds deploy` — deploy to database / cloud

### Best Practices

- Define entities in `db/schema.cds`, services in `srv/*.cds`
- Use `cds.ql` for type-safe CQL queries
- Leverage built-in authentication via `@requires` annotations
- Always run `cds lint` before committing

<!-- verbosity:detail -->
### Code Examples

#### CDS Data Modelling
```cds
entity Books : managed {
  key ID     : Integer;
  title      : localized String(111);
  author     : Association to Authors;
}
```

#### Service Definition

```cds
service CatalogService @(path:'/browse') {
  @readonly entity Books as SELECT from my.Books;
}
```

<!-- verbosity:detail -->
### Preparing for CAP 10 (June 2026)

CAP 10 ships in **June 2026**. Use the April–June 2026 window to test new defaults via feature flags and migrate before enforcement.

#### Runtime requirements

- Node.js minimum becomes **22** (recommend 24 LTS); native `node:sqlite` replaces `better-sqlite3` as the default driver.
- CAP Java 5 supports Spring Boot 4 — review the [Java 4→5 migration guide](https://cap.cloud.sap/docs/java/migration#four-to-five).

#### Flags whose defaults change in CAP 10

Update code or set the flag explicitly:

- `ieee754compatible` (now `true`) — consistent Decimal/Int64 across SQLite and HANA
- `compat_srv_getters` (now `false`) — corrected `srv.entities` reflection
- `compat_texts_entities` (now `false`) — generated `.texts` entries removed
- `legacyLocking` (now `false`) — outbox no longer holds long-lived DB locks

#### Flags removed entirely in CAP 10

Code must change — flags are ignored if set:

- `service_level_restrictions` — `@requires` is now enforced on local service calls
- `consistent_params` — `req.params` always returns an array
- `compat_save_drafts` — draft SAVE handlers no longer fire on PATCH
- `compat_assert_not_null` — error code changed from `ASSERT_MANDATORY` to `ASSERT_NOT_NULL`
- `calc_elements` — calculated elements are supported for drafts unconditionally

#### Java protocol default change (breaking)

Application Services now serve only `odata-v2`, `odata-v4`, and `odata-x4`. Annotate services explicitly or set `cds.protocols.defaults` in `application.yaml` (use `["*"]` to restore previous behavior).

#### CDS / compiler 6.5+

`@restrict`, `@requires`, `@ams` on non-existent targets are now compile **errors**, not warnings. `@assert:` constraints emit specific error codes.

#### Quick audit

Scan your project for affected flags:

```bash
grep -rnE "ieee754compatible|compat_srv_getters|compat_texts_entities|legacyLocking|service_level_restrictions|consistent_params|compat_save_drafts|compat_assert_not_null|calc_elements" \
  package.json .cdsrc*.* .env 2>/dev/null
```

#### cds test 1.0

Vitest is the new primary runner (Jest/Mocha still compatible); Chai 6 is the assertion library; native `fetch()` replaces Axios for remote calls in dev (Cloud SDK still required for production BTP destinations).
