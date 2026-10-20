"""Generate internal/ui/logo.go: 散打 as a brush-textured pixel map for half-block rendering.

Each pixel is ' ' (empty), 'a'..'j' (left character) or 'A'..'J' (right character). Level 0 ('a'/'A') is
the character's base colour; levels 6-9 are warm shades (deep amber, amber, gold, pale gold) the runtime
takes from the theme. Two pixel rows make one text row.

The texture imitates a calligraphy brush: amber pools at stroke edges, each character has a diagonal
wash, streaky "bristle" noise runs through the strokes, edges are slightly ragged (dry brush) and a few
ink dots splatter nearby.

Usage: gen-logo.py FONT PX STRETCH [SEED] [AMBER_SHARE] [VAR]
The shipped logo uses the brush font Ma Shan Zheng (SIL OFL, Google Fonts):
  curl -LO https://raw.githubusercontent.com/google/fonts/main/ofl/mashanzheng/MaShanZheng-Regular.ttf
  python3 scripts/gen-logo.py MaShanZheng-Regular.ttf 34 1.4 7 0.3 > internal/ui/logo.go
  python3 scripts/gen-logo.py MaShanZheng-Regular.ttf 24 1.4 7 0.3 logoPixelsSmall > internal/ui/logo_small.go
"""
import math
import random
import sys

from PIL import Image, ImageDraw, ImageFilter, ImageFont

font_path, px, stretch = sys.argv[1], int(sys.argv[2]), float(sys.argv[3])
seed = int(sys.argv[4]) if len(sys.argv) > 4 else 7
AMBER = float(sys.argv[5]) if len(sys.argv) > 5 else 0.3  # share of each character's pixels that turn amber
VAR = sys.argv[6] if len(sys.argv) > 6 else "logoPixels"
rng = random.Random(seed)
SS = 8
big = ImageFont.truetype(font_path, px * SS)
chars = "散打"
gap = px * SS // 10

# Lay the characters out side by side, render each on its own layer.
offsets, x = [], 0
for ch in chars:
    l, t, r, b = big.getbbox(ch)
    offsets.append(x - l)
    x += (r - l) + gap
top = min(big.getbbox(ch)[1] for ch in chars)
bottom = max(big.getbbox(ch)[3] for ch in chars)
W, H = x - gap + 2 * SS, bottom - top + 2 * SS
TW, TH = round(W / SS * stretch), round(H / SS)
layers = []
for ch, ox in zip(chars, offsets):
    img = Image.new("L", (W, H), 0)
    ImageDraw.Draw(img).text((ox + SS, SS - top), ch, font=big, fill=255)
    layers.append(img.resize((TW, TH), Image.LANCZOS).point(lambda v: 255 if v > 100 else 0))

def depth_map(mask):
    """How many 3×3 erosions each pixel survives (1 = on the edge)."""
    depth = [[0] * TW for _ in range(TH)]
    cur, d = mask, 0
    while True:
        d += 1
        on = False
        for y in range(TH):
            for x in range(TW):
                if cur.getpixel((x, y)):
                    depth[y][x] = d
                    on = True
        if not on:
            return depth
        cur = cur.filter(ImageFilter.MinFilter(3))

# Value noise, stretched so it forms streaks (bristle marks).
grid = {}
def lattice(i, j):
    if (i, j) not in grid:
        grid[(i, j)] = rng.random()
    return grid[(i, j)]

def noise(x, y, sx, sy):
    fx, fy = x / sx, y / sy
    i, j = math.floor(fx), math.floor(fy)
    u, v = fx - i, fy - j
    u, v = u * u * (3 - 2 * u), v * v * (3 - 2 * v)
    a, b = lattice(i, j), lattice(i + 1, j)
    c, d = lattice(i, j + 1), lattice(i + 1, j + 1)
    return (a * (1 - u) + b * u) * (1 - v) + (c * (1 - u) + d * u) * v

def streak(x, y):
    # Long horizontal bristles plus a finer vertical grain.
    return 0.65 * noise(x, y, 9, 1.3) + 0.35 * noise(x + 50, y, 1.4, 6)

pix = [[" "] * TW for _ in range(TH)]
for k, mask in enumerate(layers):
    depth = depth_map(mask)
    xs = [x for y in range(TH) for x in range(TW) if mask.getpixel((x, y))]
    ys = [y for y in range(TH) for x in range(TW) if mask.getpixel((x, y))]
    x0, x1, y0, y1 = min(xs), max(xs), min(ys), max(ys)
    base = "a" if k == 0 else "A"
    weights = {}
    for y in range(TH):
        for x in range(TW):
            d = depth[y][x]
            if not d:
                continue
            s = streak(x, y)
            if d == 1 and s > 0.78:
                continue  # dry-brush gap on the edge
            gx, gy = (x - x0) / max(x1 - x0, 1), (y - y0) / max(y1 - y0, 1)
            # 散: turquoise top-left washing to amber bottom-right; 打: amber top-left to deeppink.
            wash = (gx + gy) / 2 if k == 0 else 1 - (gx + gy) / 2
            edge = {1: 0.45, 2: 0.2}.get(d, 0.0)
            weights[(x, y)] = 0.5 * wash ** 1.6 + edge + 0.55 * (s - 0.5)
    # Two-toned like ink with a second colour, not an even blend (half-way mixes look muddy):
    # the top AMBER share of pixels by weight become warm shades (levels 6-9), the rest stay pure base colour.
    ranked = sorted(weights.values())
    cut = ranked[int(len(ranked) * (1 - AMBER))]
    hi = ranked[-1]
    for (x, y), w in weights.items():
        if w < cut:
            level = 0
        else:
            # Gold-leaf shading: a second, finer noise picks one of four warm shades (6 deep amber,
            # 7 amber, 8 gold, 9 pale gold), so the amber isn't one flat colour.
            g = noise(x + 200, y + 200, 2.5, 2.0)
            level = 6 + min(3, int(g * 4))
        pix[y][x] = chr(ord(base) + level)
    # A few ink splatters just outside the character, near its lower edge.
    for _ in range(4):
        for _try in range(50):
            x = rng.randint(x0 - 3, x1 + 3)
            y = rng.randint((y0 + y1) // 2, min(y1 + 2, TH - 1))
            if not (0 <= x < TW) or pix[y][x] != " ":
                continue
            near = any(0 <= y + dy < TH and 0 <= x + dx < TW and pix[y + dy][x + dx] != " "
                       for dy in (-3, 0, 3) for dx in (-3, 0, 3))
            touching = any(0 <= y + dy < TH and 0 <= x + dx < TW and pix[y + dy][x + dx] != " "
                           for dy in (-1, 0, 1) for dx in (-1, 0, 1))
            if near and not touching:
                pix[y][x] = chr(ord(base) + 9)
                break

rows = ["".join(r).rstrip() for r in pix]
while rows and not rows[0].strip():
    rows.pop(0)
while rows and not rows[-1].strip():
    rows.pop()
indent = min(len(r) - len(r.lstrip()) for r in rows if r.strip())
rows = [r[indent:] for r in rows]
if len(rows) % 2:
    rows.append("")

name = font_path.rsplit("/", 1)[-1].rsplit(".", 1)[0]
print("// Code generated by scripts/gen-logo.py; DO NOT EDIT.")
print()
print("package ui")
print()
print(f"// {VAR} is 散打 ({name}, {px}px, {stretch}× wide, seed {seed}) as a brush-textured pixel map:")
print("// 'a'..'j' left character, 'A'..'J' right character; the letter is how far its colour mixes toward")
print("// the warm accent (0-9). Two rows make one text row.")
print(f"var {VAR} = []string{{")
for r in rows:
    print(f'\t"{r}",')
print("}")
