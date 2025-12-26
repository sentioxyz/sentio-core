#!/bin/bash

set -e
rm -rf dist
mkdir dist

pnpm i
pnpm build:deps

# pnpm build:firefox
# zip dist/sentio-firefox.zip -r manifest.json out images
pnpm build
zip dist/sentio-chrome.zip -r manifest.json out images
