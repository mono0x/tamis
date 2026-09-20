#!/bin/sh
set -eu

checker=$1
shift
if [ "$checker" = vet ]; then
  set -- go vet "$@"
else
  # Build the analyzer for the host before selecting the code's target OS.
  analyzer=$(GOOS=$(go env GOHOSTOS) GOARCH=$(go env GOHOSTARCH) go tool -n "$checker")
  set -- "$analyzer" "$@"
fi

# Match the CI runners; disable cgo so analysis needs no cross C compiler.
for target in darwin/arm64 linux/amd64; do
  echo "Checking $target: $checker"
  GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 "$@"
done
