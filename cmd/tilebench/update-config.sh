#!/bin/bash

# Update config.json with all .config.json files from configs directory

set -euo pipefail

cd "$(dirname "$0")"

find configs -name "*.config.json" -printf '%f\n' | sort | jq -R . | jq -s '{files: .}' > config.json

echo "✓ Updated config.json with $(jq '.files | length' config.json) files"