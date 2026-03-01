# Docs

Project documentation lives here.

- `security-vulnerability-overrides.json` stores temporary vulnerability risk acceptances for `govulncheck` policy gates
- `complexity-baseline-issue-22.md` tracks baseline complexity violations for the refactoring effort

`security-vulnerability-overrides.json` schema reference:

- Required fields per override:
  - `id`: vulnerability ID, usually `GO-...` or `CVE-...`
  - `reason`: business or technical justification
  - `expires_on`: expiration date in `YYYY-MM-DD`
  - `owner`: GitHub username or team accountable for the override
  - `tracking_ticket`: linked issue or ticket ID
  - `scope`: affected component, version, or path description
- Optional fields per override:
  - `approved_by`: reviewer or security approver
  - `approved_date`: approval date in `YYYY-MM-DD`
  - `severity`: one of `LOW`, `MEDIUM`, `HIGH`, `CRITICAL`, `UNKNOWN`

Validation rules enforced by `backend/cmd/vulnpolicy/main.go`:

- Required string fields are trimmed and must be non-empty
- `expires_on` and `approved_date` use strict `YYYY-MM-DD` parsing
- `severity` is case-insensitive on input and must match accepted levels

Example:

```json
{
  "overrides": [
    {
      "id": "CVE-2026-1000",
      "reason": "Temporary exception until upstream fix release",
      "expires_on": "2026-04-30",
      "owner": "@plato-platform",
      "tracking_ticket": "SEC-3001",
      "scope": "frontend/package-lock.json",
      "approved_by": "@plato-security",
      "approved_date": "2026-03-01",
      "severity": "MEDIUM"
    }
  ]
}
```
