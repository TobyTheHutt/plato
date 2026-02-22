# Scripts

Development helper scripts live here.

- `dev.sh` prints local startup commands
- `check_make_dry_run.sh` validates `make --dry-run check` and fails on undefined-variable warnings
- `check_vuln.sh` runs `govulncheck` and applies Plato severity policy with override support
- `check_backend_coverage.sh` runs backend tests with Go coverage and fails if total statements are below the configured threshold
