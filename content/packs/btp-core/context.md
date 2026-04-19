## SAP Business Technology Platform (BTP)

### Overview

SAP BTP is the unified platform for building, extending, and integrating SAP applications.
It provides runtimes (Cloud Foundry, Kyma/Kubernetes, ABAP), services, and tools for SAP developers.

### Key Concepts
- **Global Account** → **Subaccount** → **Space** (Cloud Foundry) or **Namespace** (Kyma)
- **Entitlements** — quota allocations for services per subaccount
- **Service Marketplace** — catalog of all available BTP services
- **BTP CLI** (`btp`) — command-line tool for BTP account management
- **Common BTP Services** — SAP HANA Cloud, XSUAA, Connectivity Service, Destination Service

### Best Practices
- Use service instances, not user-provided services, for managed service bindings
- Set up a dedicated subaccount per environment (dev/test/prod)
- Use the BTP CLI for scripting and CI/CD pipelines
- Monitor entitlement consumption regularly

<!-- verbosity:detail -->
### Code Examples

#### Cloud Foundry Quick Reference
```bash
cf login -a <api-endpoint>
cf push <app-name> --no-start
cf bind-service <app> <service-instance>
cf start <app-name>
```
