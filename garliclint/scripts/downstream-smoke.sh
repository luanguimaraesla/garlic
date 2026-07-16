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

mkdir "$tmpdir/downstream"
cat > "$tmpdir/downstream/go.mod" <<EOF
module example.com/downstream

go 1.25.11

require github.com/luanguimaraesla/garlic v0.0.0

replace github.com/luanguimaraesla/garlic => $repo_root
EOF
cat > "$tmpdir/downstream/main.go" <<'EOF'
package downstream

import (
	"strconv"

	_ "github.com/luanguimaraesla/garlic/errors"
)

func bad() error {
	_, err := strconv.Atoi("x")
	return err
}
EOF

output="$tmpdir/output"
set +e
(
	cd "$tmpdir/downstream"
	go mod tidy
	"$linter" ./...
) >"$output" 2>&1
status=$?
set -e

cat "$output"
if ! grep -q '\[G0.01\]' "$output"; then
	echo "garliclint did not report G0.01" >&2
	exit 1
fi
if [ "$status" -eq 0 ]; then
	echo "garliclint exited zero despite findings" >&2
	exit 1
fi

