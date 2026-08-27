.PHONY: dev dev-server dev-web build build-web build-demo build-dev test test-integration coverage lint generate migrate admin admin-run admin-install check-i18n

# Run both server and web dev servers concurrently
dev:
	$(MAKE) -j2 dev-server dev-web

# Run the Go backend with dev build tag (uses Vite proxy, no embedded SPA)
dev-server:
	air --build.cmd "go build -tags=dev,demo -o bin/damask-server-demo ./cmd/server" --build.entrypoint "./bin/damask-server-demo" --build.include_dir cmd,internal --build.exclude_dir cmd/server/web --build.stop_on_error "true"

# Run the Svelte frontend
dev-web:
	cd cmd/server/web && npm run dev

# Build the frontend (SvelteKit)
build-web:
	cd cmd/server/web && npm install && npm run build

# Build the Go server binary (includes embedded SPA)
build: build-web
	go build -o bin/damask-server ./cmd/server

# Build the Go server binary with demo jobs
build-demo: build-web
	go build -tags=demo -o bin/damask-server-demo ./cmd/server

# Build the Go server binary
build-dev: build-web
	go build -o bin/damask-server-dev ./cmd/server

# Run all code generation: sqlc queries, Swagger docs, TypeScript API types
generate:
	sqlc generate -f internal/db/sqlc.yaml
	swag init -g cmd/server/main.go -o internal/docs --requiredByDefault
	npx --yes openapi-typescript@5 internal/docs/swagger.json \
	    --output cmd/server/web/src/lib/api/types.gen.ts \
	    --immutable-types

# Run all tests (excludes integration tests)
test:
	go test ./...

# Run integration tests (internal/api handlers against real SQLite)
test-integration:
	go test -tags=integration ./internal/api/... ./internal/jobs/

# Produce merged HTML coverage report (unit + integration)
coverage:
	@mkdir -p coverage
	go test -coverprofile=coverage/unit.out ./...
	go test -tags=integration -coverprofile=coverage/integration.out ./internal/api/...
	@echo "mode: set" > coverage/merged.out
	@awk 'FNR==1{next} {print}' coverage/unit.out coverage/integration.out \
	  | sort -k1,1 | awk '{ \
	      key = $$1 " " $$2; \
	      if (key in hit) { if ($$3 > hit[key]) hit[key]=$$3 } \
	      else { hit[key]=$$3; order[++n]=key } \
	    } \
	    END { for(i=1;i<=n;i++) print order[i], hit[order[i]] }' \
	  >> coverage/merged.out
	@grep -vE '^(damask/server/internal/admin|damask/server/internal/db/gen|damask/server/internal/docs|damask/server/internal/testutil|damask/server/internal/testhelpers|damask/server/internal/repository/memory|damask/server/cmd/server/web/node_modules)/' \
	  coverage/merged.out > coverage/merged.filtered.out && mv coverage/merged.filtered.out coverage/merged.out
	go tool cover -html=coverage/merged.out -o coverage/report.html
	@go tool cover -func=coverage/merged.out | tail -1
	@echo "Report: coverage/report.html"

# Run linters
lint:
	golangci-lint run --config .golangci.yaml --fix
	cd cmd/server/web && npm run check

# Apply DB migrations (for manual use; server auto-migrates on start)
migrate:
	go run ./cmd/server --migrate-only

## Admin TUI
admin:
	go build -ldflags="-s -w" -o bin/damask-admin ./cmd/admin

admin-run:
	go run ./cmd/admin

admin-install:
	go install ./cmd/admin

i18n:
	npx @inlang/paraglide-js compile --project ./project.inlang --outdir ./src/lib/paraglide

i18n-check:
	cd cmd/server/web/messages && \
	missing_found=false; \
	for file in *.json; do \
		[ "$$file" = "en.json" ] && continue; \
		missing=$$(jq -r --slurpfile ref "en.json" \
			'($$ref[0] | keys) - keys | .[]' "$$file"); \
		if [ -n "$$missing" ]; then \
			missing_found=true; \
			echo "Missing in $$file:"; \
			echo "$$missing" | sed 's/^/  - /'; \
		fi; \
	done; \
	$$missing_found || echo "All files are in sync with en.json"
