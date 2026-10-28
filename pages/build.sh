#!/bin/sh
# Assembles the GitHub Pages site in _site/: the landing page, the API reference with the one
# openapi.json the Go tests check, the demo build of the page, and the logo's one copy.
# Run from the repository's root, after `pnpm build:demo` in web/.
set -eu
rm -rf _site
mkdir -p _site/api
cp pages/index.html _site/
cp pages/api/index.html _site/api/
# The document's version is the build's, as /api/openapi.json says it; here, the commit's.
version=$(git describe --tags --always)
sed "s/\"version\": \"dev\"/\"version\": \"$version\"/" internal/transport/httpapi/openapi.json > _site/api/openapi.json
grep -q "\"version\": \"$version\"" _site/api/openapi.json
cp assets/logo.svg assets/social-preview.png _site/
cp -R web/dist-demo _site/demo
