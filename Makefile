.PHONY: check test-frontend test-backend typecheck

# Run all quality checks
check: typecheck test-frontend test-backend

# Frontend tests with coverage thresholds from Vitest config
test-frontend:
	cd frontend && npm test -- --coverage

# Backend tests with coverage reporting
test-backend:
	cd backend && go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out

# TypeScript type checking
typecheck:
	cd frontend && npx tsc --noEmit

# Optional future targets once tooling is configured:
# lint-frontend:
# 	cd frontend && npm run lint
#
# lint-backend:
# 	cd backend && golangci-lint run
#
# format-check:
# 	# add format checking commands once formatter is configured
