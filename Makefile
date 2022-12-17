.PHONY: build help generate generateAll

help:
	@echo "Makefile usage:"
	@echo -e "make [help]  \tto show this message"
	@echo -e "make build  \tto build all utilities for host arch"
	@echo
	@echo -e "./scripts/build.sh usage:"
	./scripts/build.sh -h

internal/exclude/exclude.go: internal/exclude/gen.go internal/exclude/genexclude.go
	go generate ./internal/exclude

generate: internal/exclude/exclude.go

generateAll:
	go generate ./...

build: generate
	./scripts/build.sh
