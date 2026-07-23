#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
cd "$repo_root"
tmpdir=$(mktemp -d)

cleanup() {
	find "$tmpdir" -type f -delete
	find "$tmpdir" -depth -type d -empty -delete
}
trap cleanup EXIT

linter="$tmpdir/garliclint"
go build -o "$linter" ./cmd/garliclint

mkdir -p "$tmpdir/downstream/violating" "$tmpdir/downstream/testonly" "$tmpdir/downstream/clean"
cat > "$tmpdir/downstream/go.mod" <<EOF
module example.com/downstream

go 1.25.12

require github.com/luanguimaraesla/garlic v0.0.0

replace github.com/luanguimaraesla/garlic => "$repo_root"
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
cat > "$tmpdir/downstream/testonly/main.go" <<'EOF'
package testonly
EOF
cat > "$tmpdir/downstream/testonly/main_test.go" <<'EOF'
package testonly

import "github.com/luanguimaraesla/garlic/errors"

func helper() error {
	return errors.New(errors.KindError, "test")
}

func bad() error {
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

testonly_output="$tmpdir/testonly-output"
set +e
(
	cd "$tmpdir/downstream/testonly"
	"$linter" ./...
) >"$testonly_output" 2>&1
testonly_status=$?
set -e

cat "$testonly_output"
if ! grep -q '\[G0.01\]' "$testonly_output"; then
	echo "garliclint did not report G0.01 for a test-file violation" >&2
	exit 1
fi
if [ "$testonly_status" -eq 0 ]; then
	echo "garliclint exited zero despite test-file findings" >&2
	exit 1
fi

testonly_excluded_output="$tmpdir/testonly-excluded-output"
if ! (
	cd "$tmpdir/downstream/testonly"
	"$linter" -test=false ./...
) >"$testonly_excluded_output" 2>&1; then
	cat "$testonly_excluded_output" >&2
	echo "garliclint failed when excluding test files" >&2
	exit 1
fi
if grep -q '\[G0\.' "$testonly_excluded_output"; then
	cat "$testonly_excluded_output" >&2
	echo "garliclint reported test-file diagnostics with -test=false" >&2
	exit 1
fi

violating_excluded_output="$tmpdir/violating-excluded-output"
set +e
(
	cd "$tmpdir/downstream/violating"
	"$linter" -test=false ./...
) >"$violating_excluded_output" 2>&1
violating_excluded_status=$?
set -e

cat "$violating_excluded_output"
if ! grep -q '\[G0.01\]' "$violating_excluded_output"; then
	echo "garliclint did not report G0.01 for a non-test violation with -test=false" >&2
	exit 1
fi
if [ "$violating_excluded_status" -eq 0 ]; then
	echo "garliclint exited zero despite non-test findings with -test=false" >&2
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
