# Complexity Baseline for Issue 22

Captured on 2026-03-01.

## Scope

Baseline capture before refactoring for parent issue 22.

## Target thresholds

- `cyclop`: `max-complexity=25`, `package-average=12`
- `gocyclo`: `min-complexity=20`
- `gocognit`: `min-complexity=25`

## Command used

```bash
./scripts/lint-complexity.sh --no-fail ./backend/...
```

## Baseline summary

- Total violations: `15`
- Unique functions over threshold: `7`
- Violations by linter:
  - `gocognit`: `6`
  - `gocyclo`: `6`
  - `cyclop`: `3`

## Functions over threshold

| File | Function | Violations |
| --- | --- | --- |
| `backend/internal/httpapi/routes_groups.go` | `(*API).handleGroupByID` | `gocognit=73`, `gocyclo=31`, `cyclop=33` |
| `backend/internal/domain/calc.go` | `CalculateAvailabilityLoad` | `gocognit=66`, `gocyclo=36`, `cyclop=36` |
| `backend/internal/domain/calc.go` | `selectedPeopleForScope` | `gocognit=43`, `gocyclo=21` |
| `backend/internal/httpapi/routes_organisations.go` | `(*API).handleOrganisationByID` | `gocognit=42`, `gocyclo=21` |
| `backend/internal/service/service_allocations.go` | `(*Service).validateAllocationLimit` | `gocognit=41`, `gocyclo=21` |
| `backend/internal/httpapi/routes_persons.go` | `(*API).handlePersonByID` | `gocognit=40` |
| `backend/internal/httpapi/router.go` | `(*API).ServeHTTP` | `gocyclo=28`, `cyclop=29` |

## Expected focus checks from issue context

- Confirmed: `router.go` `ServeHTTP`
- Confirmed: `service_allocations.go` `validateAllocationLimit`
- Confirmed: `domain/calc.go` `CalculateAvailabilityLoad`
- Confirmed: `domain/calc.go` `selectedPeopleForScope`
- Additional hotspots detected: group, organisation, and person route handlers
