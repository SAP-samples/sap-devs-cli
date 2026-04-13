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
