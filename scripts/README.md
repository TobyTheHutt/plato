# Scripts

Development helper scripts live here.

- `dev.sh` prints local startup commands
- `start-backend.sh` builds and starts the backend service for local development
- `start-frontend.sh` starts the frontend Vite dev server for local development
- `service-start.sh` starts a service process, tracks PID, and waits for health readiness
- `service-stop.sh` stops a tracked service process gracefully and removes PID files
- `wait-for-service.sh` waits for an HTTP endpoint to become reachable with timeout handling
- `wait-for-exit.sh` waits for a process to stop, then escalates from `SIGTERM` to `SIGKILL` when needed
- `service-status.sh` reports backend and frontend PID state and health checks
- `path-utils.sh` provides shared repo-relative path resolution helpers for scripts
- `check_make_dry_run.sh` validates `make --dry-run check` and fails on undefined-variable warnings
- `check_vuln.sh` runs `govulncheck` in source and binary modes, deduplicates binary output against source results, applies Plato severity policy with override support, and can write JSON reports via `PLATO_VULN_REPORT_DIR` (relative paths resolve from repository root, CI defaults to `.cache/vuln/reports`)
- `check_go_toolchain.sh` verifies local Go exactly matches `backend/go.mod` and prints a clear mismatch error
- `check_backend_coverage.sh` runs backend tests with Go coverage and fails if total statements are below the configured threshold
