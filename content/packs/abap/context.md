## ABAP Cloud

ABAP Cloud is SAP's approach to ABAP development for SAP BTP and S/4HANA Cloud public edition.
It enforces clean-core principles — only released APIs, no modifications to SAP standard objects.

### Key Concepts
- **ABAP Development Tools (ADT)** — Eclipse-based IDE for ABAP Cloud development
- **Tier-1 APIs** — SAP-released stable APIs for ABAP Cloud; use these instead of internal function modules
- **ABAP RESTful Application Programming Model (RAP)** — the recommended framework for building SAP Fiori apps and OData services in ABAP Cloud
- **Business Technology Platform (BTP) ABAP Environment** — steampunk; a managed ABAP runtime on SAP BTP

### RAP Quick Reference
- Business Objects: define with CDS views + behaviour definition
- Service Binding: expose as OData V2/V4
- Draft handling: built-in with `with draft` in behaviour definition

### Best Practices
- Always check S/4HANA API compatibility before using a function module
- Use CDS-based views instead of direct table selects
- Leverage the ABAP Test Cockpit (ATC) for code quality checks
- Prefer released APIs over direct system calls
