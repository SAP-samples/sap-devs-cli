## Use btp CLI for automation
Tags: btp,cli,automation
The `btp` CLI supports all BTP cockpit operations and can be scripted for CI/CD pipelines. Run `btp login` once then use service-key and subaccount commands.

## Check entitlements before deploying
Tags: btp,entitlements
Always verify service entitlements are assigned to your subaccount before deploying. Missing entitlements cause cryptic deployment errors.

## Use service bindings not hardcoded credentials
Tags: btp,security,cloud-foundry
Bind services to your CF app — credentials are injected via VCAP_SERVICES. Never hardcode service credentials.

## Kyma vs Cloud Foundry — choose by workload
Tags: btp,kyma,cloud-foundry
Use Kyma (Kubernetes) for containerised workloads and microservices. Use Cloud Foundry for managed PaaS deployments of CAP or Node.js apps.
