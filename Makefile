.PHONY: help

HELP_FUN = %help; while (<>) { /^([A-Za-z0-9_-]+)\s*:.*\#\#(?:@([A-Za-z0-9_-]+))?\s(.*)$$/ or next; push @{$$help{$$2 || "other"}}, [$$1, $$3]; $$width = length($$1) if length($$1) > $$width } print "\n"; for $$category (sort keys %help) { print "\e[37m$$category\e[0m\n"; for $$entry (@{$$help{$$category}}) { printf "  \e[33m%-*s\e[0m  \e[32m%s\e[0m\n", $$width, $$entry->[0], $$entry->[1] } }

help: ##@other Show this help.
	@perl -e '$(HELP_FUN)' $(MAKEFILE_LIST)

##@build
build: ##@build Build all packages.
	go build ./...

##@quality
test: ##@quality Run the test suite.
	go test ./...

vet: ##@quality Run Go vet.
	go vet ./...

##@release
release-check: test vet build ##@release Run the release validation checks.
