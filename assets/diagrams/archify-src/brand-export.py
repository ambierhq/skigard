#!/usr/bin/env python3
"""Brand-tune an Archify SVG export: skigard palette, no legend/lane frames, tight crop.
usage: build.py <in.svg> <dark|light> <out.svg>"""
import re, sys
src, theme, out = sys.argv[1:4]
s = open(src).read()
P = {
    'dark':  dict(bg='#16130E', fg='#F4EEE2', dim='#B7A98C', acc='#5AA9FF', onacc='#16130E', node='#26211A', stroke='#6B6150', arrow='#B7A98C'),
    'light': dict(bg='#F4EEE2', fg='#16130E', dim='#5E5440', acc='#2B6CB0', onacc='#FFFFFF', node='#FBF8F1', stroke='#A39777', arrow='#6E6450'),
}[theme]
# legend, lane frames, grid
s = re.sub(r'<!-- Legend -->.*?(</svg>)', r'\1', s, flags=re.S)
s = re.sub(r'<rect data-graph-role="structural-frame"[^>]*/>\s*<text[^>]*>[^<]*</text>', '', s)
s = s.replace('<rect width="100%" height="100%" fill="url(#grid)"/>', '')
X, Y, W, H = 30, 72, 748, 214
s = re.sub(
    r'<rect x="0" y="0" width="788" height="\d+" fill="#[0-9a-fA-F]+"/>',
    f'<rect x="{X}" y="{Y}" width="{W}" height="{H}" rx="12" fill="{P["bg"]}"/>',
    s, count=1)
head = re.search(r'<svg[^>]*>', s).group(0)
h2 = re.sub(r'viewBox="[^"]*"', f'viewBox="{X} {Y} {W} {H}"', head)
h2 = re.sub(r'width="\d+"', f'width="{W}"', h2)
h2 = re.sub(r'height="\d+"', f'height="{H}"', h2)
h2 = re.sub(r'style="[^"]*"', '', h2)
h2 = re.sub(r'data-reader-fit="[^"]*"', '', h2)
s = s.replace(head, h2, 1)
DESC = '<desc id="archify-diagram-description">skigard add, quarantine, scan and score, agent review, approve (automatic above the threshold, otherwise a human), skigard.lock, install; update loops back to quarantine and verify checks drift.</desc>'
s = re.sub(r'<desc id="archify-diagram-description">[^<]*</desc>', DESC, s)
css = f'''
svg [data-node-id] > rect:not(.c-mask) {{ fill: {P["node"]} !important; stroke: {P["stroke"]} !important; stroke-width: 1.25px; }}
svg .c-mask {{ fill: {P["bg"]}; }}
svg .t-primary {{ fill: {P["fg"]}; }}
svg .t-muted, svg .t-security {{ fill: {P["dim"]}; }}
svg .semantic-sigil {{ stroke: {P["dim"]}; color: {P["dim"]}; }}
svg .a-default, svg .a-dashed {{ stroke: {P["arrow"]}; }}
svg .m-default, svg .m-dashed {{ fill: {P["arrow"]}; }}
svg .a-emphasis {{ stroke: {P["acc"]}; stroke-width: 2.6px; }}
svg .m-emphasis {{ fill: {P["acc"]}; }}
svg [data-edge-from] > rect.c-mask {{ fill: {P["bg"]}; stroke: {P["stroke"]}; }}
svg [data-node-id="approve"] > rect:not(.c-mask) {{ fill: {P["acc"]} !important; stroke: {P["acc"]} !important; stroke-width: 2.5px; }}
svg [data-node-id="approve"] text {{ fill: {P["onacc"]} !important; }}
svg [data-node-id="approve"] .semantic-sigil {{ stroke: {P["onacc"]}; color: {P["onacc"]}; }}
svg [data-node-id] text.t-muted {{ font-size: 8.6px; }}
svg [data-node-id] text.t-security {{ font-size: 8.2px; }}
svg [data-animate] {{ animation: none !important; }}
'''
s = s.replace('</svg>', f'<style>{css}</style></svg>', 1)
open(out, 'w').write(s)
