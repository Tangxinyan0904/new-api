#!/usr/bin/env sh
# Builds the new-api Docker image with the version the console shows next to
# the site name: the build date in Beijing time and the commit, for example
# 2026.10.20-26844859. A build from uncommitted or new files ends in -dirty.
#
# Usage: bin/build-niko-image.sh [image tag]
# The tag defaults to nikoapi-new-api:<version>.
set -eu
cd "$(dirname "$0")/.."

# CST-8 is Beijing time without needing the zoneinfo files.
version="$(TZ=CST-8 date +%Y.%m.%d)-$(git rev-parse --short=8 HEAD)"
if [ -n "$(git status --porcelain -- . ':(exclude)VERSION')" ]; then
	version="$version-dirty"
fi
tag="${1:-nikoapi-new-api:$version}"

# The Dockerfile stamps the VERSION file into the backend and the web build.
# Put the empty file back afterwards, also when the build is interrupted.
trap 'git checkout -- VERSION' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP
printf '%s' "$version" > VERSION
docker build -t "$tag" .
echo "Built $tag (version $version)"
