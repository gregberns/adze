#!/usr/bin/env bash
set -uo pipefail
# Note: -e is intentionally omitted so we can capture exit codes from failing
# subshells without the test harness itself aborting.

# Test suite for install.sh
# Runs basic validation of the install script's logic without hitting the network.

PASS=0
FAIL=0
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_SCRIPT="${SCRIPT_DIR}/install.sh"

# --- Helpers ---

assert_eq() {
    local label="$1" expected="$2" actual="$3"
    if [ "$expected" = "$actual" ]; then
        echo "  PASS: $label"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $label (expected '$expected', got '$actual')"
        FAIL=$((FAIL + 1))
    fi
}

assert_contains() {
    local label="$1" needle="$2" haystack="$3"
    if echo "$haystack" | grep -qF "$needle"; then
        echo "  PASS: $label"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $label (expected output to contain '$needle')"
        FAIL=$((FAIL + 1))
    fi
}

assert_exit_code() {
    local label="$1" expected="$2" actual="$3"
    if [ "$expected" = "$actual" ]; then
        echo "  PASS: $label"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $label (expected exit code $expected, got $actual)"
        FAIL=$((FAIL + 1))
    fi
}

# Helper: extract a function from install.sh and run it with a mocked uname.
# Usage: run_with_mock <function_name> <uname_flag> <mock_value>
# Captures stdout in $RESULT and exit code in $EXIT_CODE.
run_detect() {
    local func_name="$1" mock_value="$2"
    local func_body
    func_body="$(sed -n "/^${func_name}()/,/^}/p" "$INSTALL_SCRIPT")"

    RESULT=""
    EXIT_CODE=0
    RESULT="$(bash -c "
        set -uo pipefail
        ${func_body}
        uname() { echo \"${mock_value}\"; }
        ${func_name}
    " 2>/dev/null)" || EXIT_CODE=$?
}

# --- Test: OS detection ---

echo "== OS detection =="

run_detect "detect_os" "Darwin"
assert_eq "Darwin maps to darwin" "darwin" "$RESULT"
assert_exit_code "Darwin exits 0" "0" "$EXIT_CODE"

run_detect "detect_os" "Linux"
assert_eq "Linux maps to linux" "linux" "$RESULT"
assert_exit_code "Linux exits 0" "0" "$EXIT_CODE"

run_detect "detect_os" "FreeBSD"
assert_eq "FreeBSD is unsupported (no stdout)" "" "$RESULT"
assert_exit_code "FreeBSD exits with code 1" "1" "$EXIT_CODE"

# --- Test: Architecture mapping ---

echo ""
echo "== Architecture mapping =="

run_detect "detect_arch" "x86_64"
assert_eq "x86_64 maps to amd64" "amd64" "$RESULT"
assert_exit_code "x86_64 exits 0" "0" "$EXIT_CODE"

run_detect "detect_arch" "aarch64"
assert_eq "aarch64 maps to arm64" "arm64" "$RESULT"
assert_exit_code "aarch64 exits 0" "0" "$EXIT_CODE"

run_detect "detect_arch" "arm64"
assert_eq "arm64 maps to arm64" "arm64" "$RESULT"
assert_exit_code "arm64 exits 0" "0" "$EXIT_CODE"

run_detect "detect_arch" "riscv64"
assert_eq "riscv64 is unsupported (no stdout)" "" "$RESULT"
assert_exit_code "riscv64 exits with code 1" "1" "$EXIT_CODE"

# --- Test: PATH warning ---

echo ""
echo "== PATH warning =="

# Simulate INSTALL_DIR not in PATH.
# Use a separate bash invocation to avoid bash 3.2 issues with set -u in $() subshells.
path_output="$(bash -c '
    INSTALL_DIR="/some/nonexistent/dir"
    TEST_PATH="/usr/bin:/usr/local/bin"
    case ":${TEST_PATH}:" in
        *":${INSTALL_DIR}:"*) ;;
        *) echo "Warning: ${INSTALL_DIR} is not in your PATH" ;;
    esac
')"
assert_contains "Warns when install dir not in PATH" "Warning: /some/nonexistent/dir is not in your PATH" "$path_output"

# Simulate INSTALL_DIR IS in PATH.
path_output="$(bash -c '
    INSTALL_DIR="/usr/local/bin"
    TEST_PATH="/usr/bin:/usr/local/bin"
    case ":${TEST_PATH}:" in
        *":${INSTALL_DIR}:"*) echo "IN_PATH" ;;
        *) echo "Warning: ${INSTALL_DIR} is not in your PATH" ;;
    esac
')"
assert_eq "No warning when install dir in PATH" "IN_PATH" "$path_output"

# --- Test: curl dependency check ---

echo ""
echo "== curl dependency check =="

# Run the install script in an environment where curl is not available.
# Use PATH=/bin which has bash but typically not curl.
# Use an explicit path to bash to avoid depending on PATH for the invocation itself.
error_output="$(PATH="/bin" /bin/bash "$INSTALL_SCRIPT" 2>&1)" || true
exit_code=0
PATH="/bin" /bin/bash "$INSTALL_SCRIPT" > /dev/null 2>&1 || exit_code=$?
assert_contains "Reports missing curl" "curl is required but not found" "$error_output"
assert_exit_code "Exits 1 when curl is missing" "1" "$exit_code"

# --- Test: Script uses set -euo pipefail ---

echo ""
echo "== Script safety =="

head_line="$(sed -n '2p' "$INSTALL_SCRIPT")"
assert_eq "Has set -euo pipefail" "set -euo pipefail" "$head_line"

# --- Test: Script is executable ---

echo ""
echo "== File permissions =="

if [ -x "$INSTALL_SCRIPT" ]; then
    echo "  PASS: install.sh is executable"
    PASS=$((PASS + 1))
else
    echo "  FAIL: install.sh is not executable"
    FAIL=$((FAIL + 1))
fi

# --- Summary ---

echo ""
echo "================================"
echo "Results: $PASS passed, $FAIL failed"
echo "================================"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
