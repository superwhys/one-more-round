#!/bin/sh
# Local-only DevTools API fixture; all MySQL data and photos are disposable.
# The caller must build web/dist before starting because the test embeds Web.
set -eu
cd "$(dirname "$0")/.."
OMR_MINI_FIXTURE=1 exec ./scripts/test-mysql.sh -run '^TestWechatMiniFixture$' -v -timeout 45m
