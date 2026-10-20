"""Convert Monkeytype's frontend/src/ts/constants/themes.ts (stdin) to JSON (stdout)."""
import json
import re
import sys

src = sys.stdin.read()
body = src[src.index("export const themes"):]
need = ["bg", "main", "caret", "sub", "subAlt", "text", "error", "errorExtra"]
extra = ["colorfulError", "colorfulErrorExtra"]
themes = {}
for m in re.finditer(r'(?:"([^"]+)"|([A-Za-z0-9_]+)):\s*\{([^{}]*)\}', body):
    name = m.group(1) or m.group(2)
    fields = dict(re.findall(r'(\w+):\s*"(#[0-9a-fA-F]{3,8})"', m.group(3)))
    if all(k in fields for k in need):
        themes[name] = {k: fields[k] for k in need + extra if k in fields}
json.dump(themes, sys.stdout, indent=1, sort_keys=True)
