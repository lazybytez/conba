#!/bin/sh
# Fake restic binary for tests. Behavior controlled by env vars:
#   GO_HELPER_STDOUT — written to stdout
#   GO_HELPER_STDERR — written to stderr
#   GO_HELPER_EXIT_CODE — exit code (default 0)

if [ -n "$GO_HELPER_STDOUT" ]; then
    printf '%s' "$GO_HELPER_STDOUT"
fi

if [ -n "$GO_HELPER_STDERR" ]; then
    printf '%s' "$GO_HELPER_STDERR" >&2
fi

exit "${GO_HELPER_EXIT_CODE:-0}"
