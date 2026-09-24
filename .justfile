# https://just.systems/man/en/

[private]
default:
    just --list --unsorted

# Install the toolchain pinned in .tool-versions
asdf:
    asdf install

# Build the binary (delegates to make for version ldflags)
build:
    make build

# Lint files (check-only; use lint-fix to rewrite)
lint:
    docker run --rm --volume=$PWD:$PWD:ro --workdir=$PWD git.grayc.dev/grayc-devops/woodpecker-lint:v0.4.0

# Lint files, fixing whatever the linters can fix in place.
lint-fix:
    docker run --rm --user=$(id -u):$(id -g) --volume=$PWD:$PWD --workdir=$PWD git.grayc.dev/grayc-devops/woodpecker-lint:v0.4.0 --fix

# Test the helm renderer
test:
    make test
