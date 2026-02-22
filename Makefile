.PHONY: check check-dry-run lint-makefile lint-scripts test-frontend test-backend typecheck

# Run all quality checks
check: lint-makefile lint-scripts typecheck test-frontend test-backend

# Validate target graph and command expansion without execution
check-dry-run:
	MAKE="$(MAKE)" bash ./scripts/check_make_dry_run.sh

# Makefile static analysis
lint-makefile:
	checkmake Makefile

# Shell script static analysis
lint-scripts:
	[ ! -d scripts ] || find scripts -type f -name '*.sh' -exec shellcheck {} +

# Frontend tests with coverage thresholds from Vitest config
test-frontend:
	cd frontend && CI=1 NO_COLOR=1 npm --silent test -- --coverage

# Backend tests with coverage reporting
test-backend:
	cd backend && go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out | tail -n 1

# TypeScript type checking
typecheck:
	cd frontend && npx tsc --noEmit --pretty false

# Optional future targets once tooling is configured:
# lint-frontend:
# 	cd frontend && npm run lint
#
# lint-backend:
# 	cd backend && golangci-lint run
#
# format-check:
# 	# add format checking commands once formatter is configured
