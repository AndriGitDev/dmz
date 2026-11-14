# DMZ app mark

Original project artwork, GPL-3.0-only, © 2026 dmz contributors. The geometric
`d.` monogram uses the existing DMZ/Kastro charcoal, off-white and vermilion
palette. It is an app mark, not a replacement for Kastro's corporate logo.
No downloaded logo, font, model or dataset is included.

`dmz.svg` is the editable vector master. `png/` contains committed raster exports;
`MenuBar.png` is its monochrome template for macOS. Re-render with the existing
Playwright development dependency and Chromium (no remote requests):

```
DMZ_TEST_PLAYWRIGHT=/absolute/path/to/playwright \
DMZ_TEST_BROWSER=/absolute/path/to/chromium node scripts/render-icons.mjs
python3 scripts/build-icon-resources.py
python3 scripts/check-icon-resources.py
```

The Python builder uses only the standard library. It deterministically assembles
ICNS/ICO containers and an AMD64 Windows resource object from the committed PNGs,
and copies extension sizes. The `.syso` has icon data only, no executable code.
Go links it only on Windows AMD64. `check-icon-resources.py <DMZ.exe>` independently
walks the linked PE resource directory and checks every image against its source.
macOS package checks verify the ICNS bundle reference before ad-hoc signing.
Native Finder/Explorer rendering remains part of operator review.
