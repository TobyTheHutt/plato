BACKEND_COVERAGE_THRESHOLD ?= 90
PID_DIR ?= .make
BACKEND_PID ?= $(PID_DIR)/backend.pid
FRONTEND_PID ?= $(PID_DIR)/frontend.pid
BACKEND_PID_FILE ?= $(BACKEND_PID)
FRONTEND_PID_FILE ?= $(FRONTEND_PID)
BACKEND_LOG ?= $(PID_DIR)/backend.log
FRONTEND_LOG ?= $(PID_DIR)/frontend.log
BACKEND_BIN ?= $(PID_DIR)/plato-backend
BACKEND_PORT ?= 8070
BACKEND_ADDR ?= 127.0.0.1:$(BACKEND_PORT)
BACKEND_URL ?= http://127.0.0.1:$(BACKEND_PORT)
BACKEND_HEALTH_URL ?= $(BACKEND_URL)/healthz
BACKEND_PROCESS_PATTERN ?= (^|[[:space:]])plato-backend([[:space:]]|$$)|/cmd/plato([[:space:]]|$$)
BACKEND_DEV_MODE ?= true
BACKEND_PRODUCTION_MODE ?= false
FRONTEND_HOST ?= 127.0.0.1
FRONTEND_PORT ?= 5173
FRONTEND_URL ?= http://$(FRONTEND_HOST):$(FRONTEND_PORT)
FRONTEND_HEALTH_URL ?= $(FRONTEND_URL)
FRONTEND_VITE_BIN ?= frontend/node_modules/.bin/vite
FRONTEND_PROCESS_PATTERN ?= (^|[[:space:]])vite([[:space:]]|$$)|node.*vite
SERVICE_START_TIMEOUT ?= 30
SERVICE_STOP_TIMEOUT ?= 10

export PID_DIR BACKEND_PID FRONTEND_PID BACKEND_PID_FILE FRONTEND_PID_FILE BACKEND_LOG FRONTEND_LOG BACKEND_BIN BACKEND_ADDR BACKEND_PORT BACKEND_URL BACKEND_HEALTH_URL BACKEND_PROCESS_PATTERN BACKEND_DEV_MODE BACKEND_PRODUCTION_MODE FRONTEND_HOST FRONTEND_PORT FRONTEND_URL FRONTEND_HEALTH_URL FRONTEND_VITE_BIN FRONTEND_PROCESS_PATTERN SERVICE_START_TIMEOUT SERVICE_STOP_TIMEOUT

.PHONY: check check-dry-run lint-makefile lint-scripts lint-backend lint-frontend scan-vulnerabilities test-frontend test-backend test-backend-report typecheck start stop restart status start-backend start-frontend stop-backend stop-frontend
.NOTPARALLEL: stop

# Start backend and frontend in coordinated order.
start: start-frontend
	@echo "all services started"
	@echo "   Backend:  $(BACKEND_URL)"
	@echo "   Frontend: $(FRONTEND_URL)"
	@echo "   Use 'make stop' to shut down both services"

# Stop frontend and backend in reverse order.
stop: stop-frontend stop-backend
	@echo "all services stopped"

# Restart local development services.
restart: stop start

# Print status for all managed services.
status:
	@bash ./scripts/service-status.sh

# Start backend only.
start-backend:
	@bash ./scripts/start-backend.sh

# Start frontend only after backend is available.
start-frontend: start-backend
	@bash ./scripts/start-frontend.sh

# Stop backend gracefully.
stop-backend:
	@SERVICE_PORT="$(BACKEND_PORT)" SERVICE_PROCESS_PATTERN="$(BACKEND_PROCESS_PATTERN)" bash ./scripts/service-stop.sh backend "$(BACKEND_PID)" "$(SERVICE_STOP_TIMEOUT)"

# Stop frontend gracefully.
stop-frontend:
	@SERVICE_PORT="$(FRONTEND_PORT)" SERVICE_PROCESS_PATTERN="$(FRONTEND_PROCESS_PATTERN)" bash ./scripts/service-stop.sh frontend "$(FRONTEND_PID)" "$(SERVICE_STOP_TIMEOUT)"

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
	[ ! -d scripts ] || find scripts -type f -name '*.sh' -exec shellcheck -x {} +

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
