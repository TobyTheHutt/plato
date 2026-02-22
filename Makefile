BACKEND_COVERAGE_THRESHOLD ?= 90

.PHONY: check check-dry-run lint-makefile lint-scripts lint-backend lint-frontend scan-vulnerabilities test-frontend test-backend test-backend-report typecheck

# Run all quality checks
check: lint-makefile lint-scripts lint-backend lint-frontend scan-vulnerabilities typecheck test-frontend test-backend

# Validate target graph and command expansion without execution
check-dry-run:
	MAKE="$(MAKE)" bash ./scripts/check_make_dry_run.sh

# Makefile static analysis
lint-makefile:
	checkmake Makefile

# Shell script static analysis
lint-scripts:
	[ ! -d scripts ] || find scripts -type f -name '*.sh' -exec shellcheck {} +

# Backend static analysis
lint-backend:
	cd backend && golangci-lint run -c ../.golangci.yml ./...

# Frontend static analysis
lint-frontend:
	cd frontend && npm --silent run lint

# Go dependency vulnerability scan with reachability-aware policy
scan-vulnerabilities:
	bash ./scripts/check_vuln.sh

# Frontend tests with coverage thresholds from Vitest config
test-frontend:
	cd frontend && CI=1 NO_COLOR=1 npm --silent test -- --coverage

# Backend tests with coverage threshold enforcement
test-backend:
	bash ./scripts/check_backend_coverage.sh "$(BACKEND_COVERAGE_THRESHOLD)"

# Generate backend coverage HTML report in backend/coverage.html
test-backend-report: test-backend
	cd backend && go tool cover -html=coverage.out -o coverage.html && echo "coverage report written to backend/coverage.html"

# TypeScript type checking
typecheck:
	cd frontend && npm --silent run typecheck
