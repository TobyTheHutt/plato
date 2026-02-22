# Plato

Plato is a small web app for lightweight resource planning.

It helps you track availability and load across organisations, projects, teams, and people.

- Frontend: React, TypeScript, Vite
- Backend: Go HTTP API
- License: AGPL-3.0-or-later

## Scope

Plato focuses on availability and load calculations for:
- Organisations
- Projects
- Teams or groups
- People

Capacity is modeled with:
- Employment percentage per person, for example 80%
- Project allocation percentage per person, for example 20% on Project A
- Organisation working time baselines for day, week, and year
- Organisation holidays
- Custom unavailability for groups and people

## Features

- Manage multiple organisations
- Create projects, teams or groups, and people
- Set employment percentage for each person
- Set project allocations for each person
- Define baseline hours for 100% day, week, and year
- Maintain calendars at organisation, group, and person level
- Calculate availability and load by day, week, month, or year

## Domain terms

- Organisation: defines working time baselines
- Person: belongs to one organisation and has an employment percentage
- Team or Group: set of people in the same organisation
- Project: belongs to one organisation
- Allocation: links a person to a project with an involvement percentage
- Calendar: organisation calendar with optional group or person overrides

## Architecture

### Stateless backend

- The backend keeps no in-memory session state
- Horizontal scaling is expected
- Durable data is handled through a persistence adapter

### Extensibility

The backend uses clear boundaries so integrations can change over time:
- Persistence can change without touching core domain logic
- Storage can be added later for imports and exports
- Authentication stays behind middleware boundaries
- Telemetry is isolated behind instrumentation boundaries
- Import and export support can be added for CSV and JSON formats

### Security baseline

- Tenant data is scoped by `organisation_id`
- Authorization checks are enforced at API boundaries
- Authentication provider choice is replaceable and separate from domain logic

## Repository layout

```text
/frontend   React SPA with Vite
/backend    Go service with domain, API, and adapters
/docs       Minimal documentation
/scripts    Development helpers
```

### Backend domain layout

Service logic is organized by domain in `backend/internal/service`:
- `service_organisations.go`
- `service_persons.go`
- `service_projects.go`
- `service_groups.go`
- `service_allocations.go`
- `service_calendar.go`
- `service_reports.go`

Router logic is organized by domain in `backend/internal/httpapi`:
- `routes_organisations.go`
- `routes_persons.go`
- `routes_projects.go`
- `routes_groups.go`
- `routes_allocations.go`
- `routes_reports.go`

## Development

### Prerequisites

- Node.js LTS and npm or pnpm
- Go stable release

### Run in development

Frontend:

```bash
cd frontend
npm install
npm run dev
```

Backend:

```bash
cd backend
go run ./cmd/plato
```

### Backend environment

- `PLATO_ADDR` default `:8070`
- `PLATO_DATA_FILE` default `./plato_runtime_data.json`
- `PLATO_DEV_USER_ID` default `dev-user`
- `PLATO_DEV_ORG_ID` default empty
- `PLATO_DEV_ROLES` default `org_admin`

The frontend uses these dev auth headers on each request:
- `X-User-ID`
- `X-Org-ID`
- `X-Role`

### Demo seed data

A ready to use demo dataset is available at `backend/demo-data.json`.

Run backend with seed data:

```bash
cd backend
PLATO_DATA_FILE=./demo-data.json go run ./cmd/plato
```

Select `Demo Org` in the frontend tenant selector.

## Testing

Coverage thresholds are enforced for frontend tests:
- Lines: 90%
- Statements: 90%
- Functions: 90%
- Branches: 80%

### Unified quality checks

Run quality checks from the repository root with `make`.

Run this before every push:

```bash
make check
```

Available targets:
- `make check` runs all quality checks in one command
- `make check-dry-run` validates make dependencies and command expansion without executing commands
- `make lint-makefile` runs `checkmake` on `Makefile`
- `make lint-scripts` runs `shellcheck` on scripts in `scripts/`
- `make lint-backend` runs `golangci-lint` on the Go backend with `.golangci.yml`
- `make typecheck` runs TypeScript type checking with `tsc --noEmit`
- `make test-frontend` runs Vitest with coverage
- `make test-backend` runs Go tests with coverage reporting

If you want to run checks directly without `make`:

Backend:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
cd backend
golangci-lint run -c ../.golangci.yml ./...
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Frontend:

```bash
cd frontend
npm test -- --coverage
```

### Makefile and shell boundaries

Use this separation of concerns to keep orchestration maintainable:

Keep in `Makefile`:
- Target and dependency wiring
- Single tool invocations
- One line command orchestration

Extract to `scripts/*.sh`:
- Conditional logic
- Loops and parsing
- Error handling or reusable procedures

Examples:

```makefile
test-frontend:
	cd frontend && npm test -- --coverage

deploy:
	./scripts/deploy.sh "$(ENV)"
```

Local quality workflow:

```bash
make check-dry-run
make --warn-undefined-variables check
```

`make check-dry-run` hard-fails when GNU Make prints undefined-variable warnings.

CI rationale:
- CI runs `make --warn-undefined-variables check` twice on purpose
- The first run validates correctness
- The second run, followed by `git diff --exit-code`, catches non-idempotent targets and tracked file side effects

### Frontend test boundaries

Use these scopes to avoid overlap and keep maintenance cost low:

| File | Scope | Typical assertion style |
| --- | --- | --- |
| `frontend/src/App.helpers.test.ts` | Unit tests for helper functions only | Pure input and output checks |
| `frontend/src/App.test.tsx` | Focused component behavior and panel-level integration | One behavior or edge case per test |
| `frontend/src/App.flows.test.tsx` | Multi-step workflows that cross panels | Journey milestones and outcomes |

Placement rule:
- If a test validates one behavior or one failure path, place it in `frontend/src/App.test.tsx`
- If a test needs multiple panels and sequential user actions, place it in `frontend/src/App.flows.test.tsx`
- If a test has no UI rendering, place it in `frontend/src/App.helpers.test.ts`

When writing frontend tests, import shared domain types from `frontend/src/app/types.ts` (or the equivalent re-exports in `frontend/src/App.tsx`) instead of redefining local copies.
Use shared mock helpers from `frontend/src/test-utils/mocks.ts` for `jsonResponse`, `textResponse`, and `buildMockAPI` to keep response behavior and fixture data consistent across test files.

## License

AGPL-3.0-or-later. See `LICENSE`.
