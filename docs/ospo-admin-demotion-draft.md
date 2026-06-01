# OSPO Control 7 — Admin Demotion Draft

**Status:** Draft — pending project owner action. **Nothing is executed by the OSPO hardening PR.**

OSPO Hardening Control 7 ("Access Restrictions") flagged 25 admins on this repository, well above the least-privilege threshold of 3–5. This document classifies the roster and proposes per-account actions.

## Bucket 1 — Project leads (keep admin)

| Login | Rationale |
|---|---|
| `qmacro` | Project lead |
| `jung-thomas` | Project owner |
| `ajmaradiaga` | Active maintainer (Developer Advocacy) |

## Bucket 2 — Org-mandated (cannot demote at repo level)

These admins are inherited from SAP organization-level membership or are platform automation. Removing them from the repo collaborator list does not actually remove their admin powers; that requires an OSPO ticket or org-admin action.

- `SAP-OSPO-ADMIN`
- `sap-ospo-bot`
- `SebastianWolf-SAP`
- `nicoschoenteich`
- `christianneu`
- `dellagustin-sap`

**Action:** verify with OSPO. If the inherited grant is intentional, leave alone. If a subset can be pruned via OSPO ticket, file one.

## Bucket 3 — Inherited / unclear (propose demote to `maintain` or `write`)

These accounts have admin but no obvious active stake in this repo. Recommended action: demote each to `maintain` (still allows triaging issues, accepting PRs, managing labels) or `write` (just code contribution). Confirm per-account before demoting; demotion notifies the user.

| Login | Suggested role after demotion |
|---|---|
| `akula86` | maintain |
| `ihrigb` | maintain |
| `vipinvkmenon` | maintain |
| `Sygyzmundovych` | maintain |
| `KevinRiedelsheimer` | maintain |
| `rbrainey` | maintain |
| `thecodester` | maintain |
| `ajinkyapatil8190` | maintain |
| `Shegox` | maintain |
| `rich-heilman` | maintain |
| `noravth` | maintain |
| `sheenamk` | maintain |
| `PoojaGidaveer` | maintain |
| `btbernard` | maintain |
| `ajaysoreng` | maintain |
| `neelamegams` | maintain |

## Demotion command (per account)

```bash
gh api -X PUT repos/SAP-samples/sap-devs-cli/collaborators/<login> \
  -f permission=maintain
```

Expected response: `204 No Content`. The user receives an email notification.

## Expected end state

| Bucket | Count |
|---|---|
| Admin (project leads) | 3 |
| Admin (org-mandated, awaiting OSPO) | 6 |
| Demoted to `maintain` | 16 |
| **Net admin count** | **9** (or fewer once OSPO bucket is pruned) |

This brings the repo close to OSPO Control 7's least-privilege threshold (3–5) without disrupting active maintainers.
