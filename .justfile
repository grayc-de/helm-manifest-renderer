# https://just.systems/man/en/

[private]
default:
    just --list --unsorted

# Build the binary (delegates to make for version ldflags)
build:
    make build

# Lint files
lint:
    make fmt
    docker run --rm --volume=$PWD:$PWD:ro --workdir=$PWD git.grayc.dev/grayc-devops/woodpecker-lint:v0.2.0

# Lint files, fixing whatever the linters can fix in place.
lint-fix:
    docker run --rm --user=$(id -u):$(id -g) --volume=$PWD:$PWD --workdir=$PWD git.grayc.dev/grayc-devops/woodpecker-lint:v0.2.0 --fix

# Test the helm renderer
test:
    make test
