# https://just.systems/man/en/

[private]
default:
    just --list --unsorted

# Build the binary (delegates to make for version ldflags)
build:
    make build

# Lint files
lint:
    just --unstable --fmt --check --color=always --justfile=.justfile
    yamllint --format=colored --strict .
    markdownlint-cli2 --fix "**/*.md"
