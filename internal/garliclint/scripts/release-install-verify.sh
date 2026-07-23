#!/usr/bin/env bash
set -euo pipefail

# Verifies a freshly released tag end to end, the way a downstream consumer
# would: install cmd/garliclint@<tag> through the examples Makefile snippet,
# then assert the pinned binary reports the tag, rejects an obvious G0.01
# violation, and passes clean code. A bounded retry loop absorbs module-proxy
# propagation lag for a tag that was cut moments ago.

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
snippet="$repo_root/internal/garliclint/examples/Makefile.garlic-lint"

raw_version="${GARLIC_VERSION:-}"
version="v${raw_version#v}"
# The whitespace test rejects multiline values that would sneak a valid
# first line past a line-oriented grep and inject extra directives into
# the generated go.mod; grep -x then anchors the full remaining value.
if [ "$version" != "$(printf '%s' "$version" | tr -d '[:space:]')" ] ||
	! printf '%s' "$version" | grep -Eqx 'v[0-9]+\.[0-9]+\.[0-9]+'; then
	echo "GARLIC_VERSION must be a pinned vX.Y.Z tag (got '$raw_version')" >&2
	exit 1
fi

retries="${RETRIES:-10}"
sleep_seconds="${SLEEP_SECONDS:-30}"
if ! printf '%s\n' "$retries" | grep -Eq '^[1-9][0-9]*$'; then
	echo "RETRIES must be a positive integer (got '$retries')" >&2
	exit 1
fi
if ! printf '%s\n' "$sleep_seconds" | grep -Eq '^[0-9]+$'; then
	echo "SLEEP_SECONDS must be a non-negative integer (got '$sleep_seconds')" >&2
	exit 1
fi

tmpdir=$(mktemp -d)

cleanup() {
	find "$tmpdir" -type f -delete
	find "$tmpdir" -depth -type d -empty -delete
}
trap cleanup EXIT

make_in() {
	local workdir=$1
	shift
	(
		cd "$workdir"
		make -f "$snippet" "$@"
	)
}

# Scratch downstream module pinned to the released tag. No replace directive:
# resolving the tag through the public proxy is exactly what is under test.
downstream="$tmpdir/downstream"
tools_dir="$tmpdir/tools"
mkdir -p "$downstream/violating" "$downstream/clean"
cat > "$downstream/go.mod" <<EOF
module example.com/downstream

go 1.25.12

require github.com/luanguimaraesla/garlic $version
EOF
cat > "$downstream/violating/main.go" <<'EOF'
package violating

import "github.com/luanguimaraesla/garlic/errors"

func helper() (int, error) {
	return 0, errors.New(errors.KindError, "tuple")
}

func bad() (int, error) {
	return helper()
}
EOF
cat > "$downstream/clean/main.go" <<'EOF'
package clean

import "github.com/luanguimaraesla/garlic/errors"

func good() error {
	return errors.New(errors.KindError, "clean")
}
EOF

attempt=1
installed=false
while [ "$attempt" -le "$retries" ]; do
	if GARLIC_VERSION="$version" GARLICLINT_TOOLS_DIR="$tools_dir" \
		make_in "$downstream" garliclint-install; then
		installed=true
		break
	fi
	echo "install attempt $attempt/$retries failed; the proxy may not serve $version yet" >&2
	attempt=$((attempt + 1))
	if [ "$attempt" -le "$retries" ]; then
		sleep "$sleep_seconds"
	fi
done
if [ "$installed" != true ]; then
	echo "FAIL: install: garliclint@$version did not install after $retries attempts" >&2
	exit 1
fi
echo "PASS: install: garliclint@$version installed via the examples Makefile"

binary="$tools_dir/garliclint-$version"
version_output=$("$binary" -version)
if [ "$version_output" != "garliclint version $version" ]; then
	echo "FAIL: version: expected 'garliclint version $version', got '$version_output'" >&2
	exit 1
fi
echo "PASS: version: pinned binary reports $version"

# The install above proxy-cached the tag, so tidy resolves the scratch
# module's dependency graph without surprises.
(
	cd "$downstream"
	go mod tidy
)

violating_output="$tmpdir/violating-output"
set +e
GARLIC_VERSION="$version" GARLICLINT_TOOLS_DIR="$tools_dir" \
	make_in "$downstream/violating" garliclint >"$violating_output" 2>&1
violating_status=$?
set -e
cat "$violating_output"
if [ "$violating_status" -eq 0 ]; then
	echo "FAIL: violation: pinned garliclint exited zero on a bare tuple return" >&2
	exit 1
fi
if ! grep -q '\[G0\.01\]' "$violating_output"; then
	echo "FAIL: violation: non-zero exit without a G0.01 finding" >&2
	exit 1
fi
echo "PASS: violation: pinned garliclint rejects a bare tuple return with G0.01"

clean_output="$tmpdir/clean-output"
if ! GARLIC_VERSION="$version" GARLICLINT_TOOLS_DIR="$tools_dir" \
	make_in "$downstream/clean" garliclint >"$clean_output" 2>&1; then
	cat "$clean_output" >&2
	echo "FAIL: clean: pinned garliclint failed on clean downstream code" >&2
	exit 1
fi
if grep -q '\[G0\.' "$clean_output"; then
	cat "$clean_output" >&2
	echo "FAIL: clean: diagnostics reported for clean downstream code" >&2
	exit 1
fi
echo "PASS: clean: pinned garliclint passes clean downstream code"

echo "release install verification passed for $version"
