#!/usr/bin/env sh
# Builds the new-api Docker image with the version the console shows next to
# the site name: the build date in Beijing time and the commit, for example
# 2026.10.20-26844859. A build from uncommitted changes ends in -dirty.
#
# Usage: bin/build-niko-image.sh [image tag]
# The tag defaults to nikoapi-new-api:<version>.
set -eu
cd "$(dirname "$0")/.."

version="$(TZ=Asia/Shanghai date +%Y.%m.%d)-$(git rev-parse --short=8 HEAD)"
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
	version="$version-dirty"
fi
tag="${1:-nikoapi-new-api:$version}"

# The Dockerfile stamps the VERSION file into the backend and the web build.
# Put the empty file back afterwards so the checkout stays clean.
trap 'git checkout -- VERSION' EXIT
printf '%s' "$version" > VERSION
docker build -t "$tag" .
echo "Built $tag (version $version)"
