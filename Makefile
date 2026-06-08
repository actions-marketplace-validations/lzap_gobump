GOTOOLCHAIN ?= go$(shell awk '/^go /{print $$2}' go.mod)

.PHONY: all fmt install tidy bump test release

all: fmt install lint

fmt:
	go fmt ./...

install:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go install ./...

lint:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go run honnef.co/go/tools/cmd/staticcheck@v0.6.1 ./...

test:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go test ./...

tidy:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go mod tidy

bump:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go run .

release:
	@bash scripts/release.sh
