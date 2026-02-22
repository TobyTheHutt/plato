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

- `PLATO_ADDR` default `:8080`
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

Coverage target is 90% or higher.

Backend:

```bash
cd backend
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Frontend:

```bash
cd frontend
npm test -- --coverage
```

When writing frontend tests, import shared domain types from `frontend/src/App.tsx` exports instead of redefining local copies.
Use shared mock helpers from `frontend/src/test-utils/mocks.ts` for `jsonResponse`, `textResponse`, and `buildMockAPI` to keep response behavior and fixture data consistent across test files.

## License

AGPL-3.0-or-later. See `LICENSE`.
