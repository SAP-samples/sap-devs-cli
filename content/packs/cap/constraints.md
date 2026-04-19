1. Never write raw SQL — always use `cds.ql` or CQL
2. Never use `req.user` without a `@requires` annotation on the service
3. Never depend on `@sap/` packages that are not publicly published on npmjs.com or not listed in the CAP released API documentation
4. Never bypass CAP's built-in authentication — use `@requires` and `@restrict` annotations
