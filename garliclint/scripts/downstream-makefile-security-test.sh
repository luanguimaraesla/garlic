#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
snippet="$repo_root/garliclint/examples/Makefile.garlic-lint"
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

assert_no_sentinel() {
	local sentinel=$1
	if [ -e "$sentinel" ]; then
		echo "unexpected command execution created $sentinel" >&2
		exit 1
	fi
}

fakebin="$tmpdir/fakebin"
mkdir -p "$fakebin"
cat > "$fakebin/go" <<'EOF'
#!/bin/sh
set -eu
printf 'install\n' >> "$GO_LOG"
if [ "${FAIL_INSTALL:-}" = true ]; then
	exit 1
fi
mkdir -p "$GOBIN"
cat > "$GOBIN/garliclint" <<'TOOL'
#!/bin/sh
exit 0
TOOL
chmod +x "$GOBIN/garliclint"
EOF
chmod +x "$fakebin/go"

# External path values must remain data, including shell substitution syntax.
for style in dollar backtick; do
	workdir="$tmpdir/tools-$style"
	sentinel="$tmpdir/tools-$style-sentinel"
	mkdir -p "$workdir"
	if [ "$style" = dollar ]; then
		payload="$tmpdir/literal-\$(touch $sentinel)"
	else
		payload="$tmpdir/literal-\`touch $sentinel\`"
	fi
	GO_LOG="$tmpdir/tools-$style-go.log" \
	PATH="$fakebin:$PATH" \
	GARLIC_VERSION=v1.2.3 \
	GARLICLINT_TOOLS_DIR="$payload" \
	make_in "$workdir" garliclint
	assert_no_sentinel "$sentinel"
done

for style in dollar backtick; do
	workdir="$tmpdir/bin-$style"
	sentinel="$tmpdir/bin-$style-sentinel"
	mkdir -p "$workdir"
	if [ "$style" = dollar ]; then
		payload="$tmpdir/missing-\$(touch $sentinel)"
	else
		payload="$tmpdir/missing-\`touch $sentinel\`"
	fi
	if GARLICLINT_BIN="$payload" make_in "$workdir" garliclint; then
		echo "invalid GARLICLINT_BIN unexpectedly succeeded" >&2
		exit 1
	fi
	assert_no_sentinel "$sentinel"
done

# A slashless override names the current-directory executable, not PATH.
workdir="$tmpdir/slashless"
pathdir="$tmpdir/conflicting-path"
mkdir -p "$workdir" "$pathdir"
cat > "$workdir/garliclint" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" > "$LOCAL_LOG"
EOF
cat > "$pathdir/garliclint" <<'EOF'
#!/bin/sh
: > "$PATH_LOG"
exit 1
EOF
chmod +x "$workdir/garliclint" "$pathdir/garliclint"
LOCAL_LOG="$tmpdir/local.log" \
PATH_LOG="$tmpdir/path.log" \
PATH="$pathdir:$PATH" \
GARLICLINT_BIN=garliclint \
make_in "$workdir" garliclint
if [ "$(cat "$tmpdir/local.log")" != "./..." ]; then
	echo "slashless GARLICLINT_BIN did not run the local executable" >&2
	exit 1
fi
if [ -e "$tmpdir/path.log" ]; then
	echo "slashless GARLICLINT_BIN ran the conflicting PATH executable" >&2
	exit 1
fi

# Default artifacts are cached per version, while explicit overrides skip install.
workdir="$tmpdir/default"
toolsdir="$tmpdir/tools"
golog="$tmpdir/go.log"
mkdir -p "$workdir"
for version in v1.2.3 v1.2.3 v1.2.4; do
	GO_LOG="$golog" \
	PATH="$fakebin:$PATH" \
	GARLIC_VERSION="$version" \
	GARLICLINT_TOOLS_DIR="$toolsdir" \
	make_in "$workdir" garliclint
done
if [ "$(wc -l < "$golog")" -ne 2 ]; then
	echo "version-qualified artifacts did not cache installs by version" >&2
	exit 1
fi

explicit="$tmpdir/explicit"
cat > "$explicit" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod +x "$explicit"
GO_LOG="$tmpdir/override-go.log" \
PATH="$fakebin:$PATH" \
GARLICLINT_BIN="$explicit" \
make_in "$workdir" garliclint
if [ -e "$tmpdir/override-go.log" ]; then
	echo "explicit GARLICLINT_BIN unexpectedly installed garliclint" >&2
	exit 1
fi

# A failed install cannot promote a stale unversioned binary.
failure_tools="$tmpdir/failure-tools"
mkdir -p "$failure_tools"
cat > "$failure_tools/garliclint" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod +x "$failure_tools/garliclint"
if GO_LOG="$tmpdir/failure-go.log" \
	FAIL_INSTALL=true \
	PATH="$fakebin:$PATH" \
	GARLIC_VERSION=v9.9.9 \
	GARLICLINT_TOOLS_DIR="$failure_tools" \
	make_in "$workdir" garliclint; then
	echo "failed garliclint installation unexpectedly succeeded" >&2
	exit 1
fi
if [ -e "$failure_tools/garliclint-v9.9.9" ]; then
	echo "failed installation promoted a stale garliclint binary" >&2
	exit 1
fi

echo "downstream Makefile security tests passed"
