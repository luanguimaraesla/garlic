#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"
tmpdir=$(mktemp -d)

cleanup() {
	find "$tmpdir" -type f -delete
	find "$tmpdir" -depth -type d -empty -delete
}
trap cleanup EXIT

linter="$tmpdir/garliclint"
go build -o "$linter" ./cmd/garliclint

mkdir -p "$tmpdir/downstream/violating" "$tmpdir/downstream/clean"
cat > "$tmpdir/downstream/go.mod" <<EOF
module example.com/downstream

go 1.25.11

require github.com/luanguimaraesla/garlic v0.0.0

replace github.com/luanguimaraesla/garlic => $repo_root
EOF
cat > "$tmpdir/downstream/violating/main.go" <<'EOF'
package violating

import "github.com/luanguimaraesla/garlic/errors"

func helper() (int, error) {
	return 0, errors.New(errors.KindError, "tuple")
}

func bad() (int, error) {
	return helper()
}
EOF
cat > "$tmpdir/downstream/clean/main.go" <<'EOF'
package clean

import "github.com/luanguimaraesla/garlic/errors"

func good() error {
	return errors.New(errors.KindError, "clean")
}
EOF

(
	cd "$tmpdir/downstream"
	go mod tidy
)

violating_output="$tmpdir/violating-output"
set +e
(
	cd "$tmpdir/downstream/violating"
	"$linter" ./...
) >"$violating_output" 2>&1
violating_status=$?
set -e

cat "$violating_output"
if ! grep -q '\[G0.01\]' "$violating_output"; then
	echo "garliclint did not report G0.01 for a tuple return" >&2
	exit 1
fi
if [ "$violating_status" -eq 0 ]; then
	echo "garliclint exited zero despite tuple-return findings" >&2
	exit 1
fi

clean_output="$tmpdir/clean-output"
if ! (
	cd "$tmpdir/downstream/clean"
	"$linter" ./...
) >"$clean_output" 2>&1; then
	cat "$clean_output" >&2
	echo "garliclint failed on clean downstream code" >&2
	exit 1
fi
if grep -q '\[G0\.' "$clean_output"; then
	cat "$clean_output" >&2
	echo "garliclint reported diagnostics for clean downstream code" >&2
	exit 1
fi
