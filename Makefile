.PHONY: install generate swagger web-build build check dev-api dev-web

VERSION ?= dev
GO ?= go
PNPM ?= pnpm

install:
	$(PNPM) --dir web install --frozen-lockfile
	$(GO) mod download

web-build: install
	$(PNPM) --dir web build

generate:
	$(GO) generate ./internal/infra/mysql

# swagger regenerates cmd/swagger/docs from the annotations in api/; the
# generated files are checked in and must not be edited by hand.
swagger:
	$(GO) generate ./cmd/swagger

build: web-build
	$(GO) build -trimpath -ldflags "-X github.com/miebyte/goutils/buildinfo.Version=$(VERSION) -X github.com/miebyte/goutils/buildinfo.ServiceName=one-more-round" -o bin/one-more-round .

check: web-build
	$(PNPM) --dir web format:check
	$(PNPM) --dir web test
	$(GO) test ./...
	$(GO) vet ./...
	git diff --check

dev-api: web-build
	$(GO) run . --debug --serviceName one-more-round

dev-web:
	$(PNPM) --dir web dev

.PHONY: integration
integration: web-build
	./scripts/test-mysql.sh

.PHONY: mini-install mini-build mini-check
mini-install:
	$(PNPM) --dir mini install --frozen-lockfile

mini-build: mini-install
	$(PNPM) --dir mini build

mini-check: mini-install
	$(PNPM) --dir mini check
