#!/bin/sh
# Refreshes the browser libraries embedded in the binary (assets/lib/*.gz). Needs npm.
# Update the versions here, in assets/index.html (CDN fallbacks) and in assets/lib/VERSIONS.txt.
set -e
cd "$(dirname "$0")"
tmp=$(mktemp -d)
(cd "$tmp" && npm init -y >/dev/null && npm i --silent marked@18.1.2 mermaid@12.2.0 d3@7.9.0 markmap-lib@0.18.12 markmap-view@0.18.12 @highlightjs/cdn-assets@11.12.0)
n="$tmp/node_modules"
gzip -9 -c "$n/marked/lib/marked.umd.js"                > assets/lib/marked.js.gz
gzip -9 -c "$n/mermaid/dist/mermaid.min.js"             > assets/lib/mermaid.js.gz
gzip -9 -c "$n/d3/dist/d3.min.js"                       > assets/lib/d3.js.gz
gzip -9 -c "$n/markmap-lib/dist/browser/index.iife.js"  > assets/lib/markmap-lib.js.gz
gzip -9 -c "$n/markmap-view/dist/browser/index.js"      > assets/lib/markmap-view.js.gz
gzip -9 -c "$n/@highlightjs/cdn-assets/highlight.min.js" > assets/lib/highlight.js.gz
rm -rf "$tmp"
echo "vendored libs refreshed"
