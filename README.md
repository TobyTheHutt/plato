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

Use the coordinated Make targets from the repository root:

```bash
make start
make status
make stop
make restart
```

Available service control targets:
- `make start-backend`
- `make start-frontend` (starts backend first if needed)
- `make stop-backend`
- `make stop-frontend`

`make start` waits for backend `GET /healthz` before starting the frontend.
Service PID files, logs, and the local backend binary are written to `.make/`.
By default, these targets run the backend with `DEV_MODE=true` and `PRODUCTION_MODE=false` for local development.

Manual fallback:

```bash
cd frontend
npm install
npm run dev
```

```bash
cd backend
go run ./cmd/plato
```

### Backend runtime mode

The backend supports explicit runtime modes:
- `DEV_MODE=true` enables development mode with header-based dev auth
- `PRODUCTION_MODE=true` enables production mode with JWT auth
- If both are unset, the backend defaults to production mode
- `DEV_MODE` and `PRODUCTION_MODE` cannot both be `true`

### Backend environment

- `PLATO_ADDR` default:
  - `127.0.0.1:8070` in development mode
  - `:8070` in production mode
- `PLATO_DATA_FILE` default `./plato_runtime_data.json`
- `PLATO_CORS_ALLOWED_ORIGINS` comma-separated origin allowlist. In production mode, wildcard `*` is rejected.
- `PLATO_AUTH_JWT_HS256_SIGNING_KEY` required in production mode

Development-mode auth settings:
- `PLATO_DEV_USER_ID` default `dev-user`
- `PLATO_DEV_ORG_ID` default empty
- `PLATO_DEV_ROLES` default `org_admin`

Production JWT requirements:
- `Authorization: Bearer <token>` header is required
- Token algorithm must be `HS256`
- Token must include `exp`
- Token must include `roles` as a comma-separated string or a string array
- User identity can be provided by `sub` or `user_id`
- Tenant scope can be provided by `org_id` or `organisation_id`

### Backend shutdown and HTTP timeouts

The backend handles `SIGINT` and `SIGTERM` with a graceful shutdown sequence:
- Stop accepting new requests
- Drain in-flight requests for up to 30 seconds
- Close repository resources before process exit

HTTP server timeouts are configured with production-safe defaults:
- `ReadHeaderTimeout`: 10 seconds, mitigates slowloris-style header attacks
- `ReadTimeout`: 15 seconds, limits slow request body uploads
- `WriteTimeout`: 15 seconds, limits slow client response reads
- `IdleTimeout`: 60 seconds, limits idle keep-alive connection buildup

For deployments and orchestrators, allow at least 30 seconds for termination so in-flight requests can complete under normal load.

Monitoring recommendations:
- Track shutdown duration from signal receipt to process exit
- Alert on repeated `server forced to shutdown` log entries
- Track spikes in request timeouts and connection reset errors during deploy windows

The frontend uses these headers in development mode:
- `X-User-ID`
- `X-Org-ID`
- `X-Role`

### Production deployment guide

1. Disable development mode
2. Set production auth and CORS settings
3. Provide JWTs from your identity provider or auth gateway

Example startup:

```bash
cd backend
PRODUCTION_MODE=true \
PLATO_AUTH_JWT_HS256_SIGNING_KEY='replace-with-strong-signing-key' \
PLATO_CORS_ALLOWED_ORIGINS='https://app.example.com,https://admin.example.com' \
go run ./cmd/plato
```

Security checklist for production:
- `DEV_MODE` is unset or `false`
- `PLATO_AUTH_JWT_HS256_SIGNING_KEY` is configured
- `PLATO_CORS_ALLOWED_ORIGINS` includes only trusted origins
- `PLATO_ADDR` is explicitly set for your deployment network topology

### Demo seed data

A ready to use demo dataset is available at `backend/demo-data.json`.

Run backend with seed data:

```bash
cd backend
PLATO_DATA_FILE=./demo-data.json go run ./cmd/plato
```

Select `Demo Org` in the frontend tenant selector.

## Testing

Coverage thresholds are enforced for frontend and backend tests.

Frontend thresholds:
- Lines: 90%
- Statements: 90%
- Functions: 90%
- Branches: 90%

Backend thresholds:
- Statements: 90%

Backend coverage workflow:
- `make test-backend` runs `go test` with `coverage.out` generation and fails if total statement coverage is below the backend threshold
- Override the threshold locally with `make test-backend BACKEND_COVERAGE_THRESHOLD=92`
- `make test-backend-report` writes `backend/coverage.html` for detailed local inspection

Frontend lint quality gates are also enforced for maintainability:
- Import hygiene via `eslint-plugin-import`, including unresolved import checks and circular dependency detection
- Accessibility checks via `eslint-plugin-jsx-a11y` recommended rules
- Security checks via `eslint-plugin-security` for risky dynamic evaluation and regular expression usage
- Complexity thresholds:
  - `complexity`: max 20 per function
  - `max-lines-per-function`: max 300, ignoring blank lines and comments
  - `max-depth`: max 4 nested blocks
  - `max-params`: max 5 function parameters

Current transitional exception:
- `frontend/src/App.tsx` has a temporary higher complexity cap of 45 and no `max-lines-per-function` cap until it is split into smaller components

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
- `make lint-frontend` runs ESLint for the React and TypeScript frontend
- `make scan-vulnerabilities` runs `govulncheck` with severity policy and accepted-risk overrides
- `make typecheck` runs TypeScript type checking with `npm run typecheck`
- `make test-frontend` runs Vitest with coverage
- `make test-backend` runs Go tests with coverage threshold enforcement
- `make test-backend-report` writes `backend/coverage.html` from `backend/coverage.out`

If you want to run checks directly without `make`:

Backend:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
cd backend
golangci-lint run -c ../.golangci.yml ./...
../scripts/check_vuln.sh
../scripts/check_backend_coverage.sh 90
go tool cover -html=coverage.out -o coverage.html
```

Frontend:

```bash
cd frontend
npm run lint
npm run typecheck
npm run test:coverage
```

### Dependency vulnerability scanning

`govulncheck` runs in CI for every pull request and every push to `main` through `make check`.

Policy:
- Reachable `HIGH` and `CRITICAL` vulnerabilities fail the build
- Reachable `MEDIUM` and `LOW` vulnerabilities emit warnings
- Reachability is taken from `govulncheck` trace data to reduce false positives

Local usage:

```bash
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
make scan-vulnerabilities
```

Accepted risk override process:
- Add temporary exceptions to `docs/security-vulnerability-overrides.json`
- Each override must include `id`, `reason`, and `expires_on`
- Overrides may use either a `GO-...` ID or a `CVE-...` alias
- Expired overrides fail the scan
- Remove overrides once fixes are released and deployed

Handling vulnerability reports:
1. Run `make scan-vulnerabilities` and capture the failing IDs
2. Upgrade to a fixed version shown by `govulncheck` output when available
3. If immediate upgrade is not possible, add a temporary override with owner, reason, and expiry
4. Open a follow-up task to remove the override before expiry
5. Follow the Go security policy for disclosure and response expectations: https://go.dev/doc/security/policy

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
	cd frontend && npm run test:coverage

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
