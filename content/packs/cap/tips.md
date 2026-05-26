## Use cds watch for local development
Tags: cap,nodejs
Run `cds watch` instead of `node server.js` — it reloads on every file change and logs all requests.

## Define managed entities for audit fields
Tags: cap,cds
Add `: managed` to your entities to get `createdAt`, `createdBy`, `modifiedAt`, `modifiedBy` for free.

## Use @readonly in service layer
Tags: cap,odata,security
Expose `@readonly` in the service layer rather than restricting at DB level — keeps schema flexible.

## Check CAP version compatibility
Tags: cap,versions
Run `cds version` to see your full CAP stack versions. Mismatched `@sap/cds` and `@sap/cds-dk` cause subtle errors.

## Audit your project for CAP 10 breaking flags
Tags: cap,migration,cap10
CAP 10 (June 2026) removes several `compat_*` flags and flips defaults. Run `grep -rnE "ieee754compatible|compat_srv_getters|compat_texts_entities|legacyLocking|service_level_restrictions|consistent_params|compat_save_drafts|compat_assert_not_null|calc_elements" package.json .cdsrc*.* .env` to find any references that need attention before upgrading.

## Pin protocols on CAP Java services before CAP 10
Tags: cap,java,cap10,odata
In CAP 10, Java Application Services serve only `odata-v2`, `odata-v4`, and `odata-x4` by default. Either annotate each service with `@odata` / `@hcql` or set `cds.protocols.defaults: ["*"]` in `application.yaml` to keep current behavior.
