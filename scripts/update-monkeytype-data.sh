#!/bin/bash
# Refresh the vendored Monkeytype data (GPL-3.0) in internal/data/assets.
set -euo pipefail
cd "$(dirname "$0")/../internal/data/assets"
B=https://raw.githubusercontent.com/monkeytypegame/monkeytype/master/frontend
curl -sfL -o quotes_english.json "$B/static/quotes/english.json"
curl -sfL -o english_200.json "$B/static/languages/english.json"
curl -sfL -o english_1k.json "$B/static/languages/english_1k.json"
curl -sfL -o english_10k.json "$B/static/languages/english_10k.json"
curl -sfL "$B/src/ts/constants/themes.ts" | python3 "$OLDPWD/$(dirname "$0")/gen-themes.py" > monkeytype_themes.json
echo "updated"
