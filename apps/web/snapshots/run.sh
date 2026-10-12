#!/bin/sh
# Runs the web UI's snapshot tests against Chromium in the Playwright image
# of @playwright/test's version, started here and stopped after. Arguments
# go to playwright test; --update-snapshots records the references.
# Usage: snapshots/run.sh [args]
set -eu
cd "$(dirname "$0")"
version=$(node -p 'require("@playwright/test/package.json").version')
# The browser server is the host's playwright-core, which the image runs.
core=$(node -p 'require("path").dirname(require.resolve("playwright-core/package.json", { paths: [require.resolve("@playwright/test")] }))')
port=3790
name=mavio-snapshots-$$
docker run -d --rm --name "$name" --init --ipc=host -p "127.0.0.1:$port:$port" \
	-v "$core:/playwright-core:ro" "mcr.microsoft.com/playwright:v$version-noble" \
	node /playwright-core/cli.js run-server --port "$port" --host 0.0.0.0 >/dev/null
trap 'docker stop "$name" >/dev/null' EXIT
# The server is up when it answers.
i=0
until curl -s -o /dev/null "http://127.0.0.1:$port/"; do
	i=$((i + 1))
	[ "$i" -lt 120 ] || { echo "the browser server did not start" >&2; exit 1; }
	sleep 1
done
MAVIO_BROWSER="ws://127.0.0.1:$port/" npx playwright test -c playwright.config.ts "$@"
