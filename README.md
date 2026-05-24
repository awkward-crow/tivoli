# tivoli

A Go static site generator that produces a single-page carousel for showcasing
GitHub repos. Works on mobile and desktop. No npm, no framework.

**Demo:** [awkward-crow.surge.sh](https://awkward-crow.surge.sh) &nbsp; ![CI](https://github.com/awkward-crow/tivoli/actions/workflows/ci.yml/badge.svg)

## what's here

```
cmd/generate/main.go      generator
templates/index.html.tmpl page template
static/style.css          CSS scroll-snap carousel
static/carousel.js        nav arrows, dots, keyboard
repos/                    one directory per repo (see below)
site/                     generated output — do not commit (gitignored)
```

## adding a repo

Create a directory under `repos/`:

```
repos/myproject/
  config.yaml       required
  description.md    optional long description (plain text, blank lines = paragraphs)
  image.webp        optional background image (first image file found is used)
```

### config.yaml

```yaml
title:
  text: My Project
  fg: 0xffffff        # optional — default white
  font: georgia       # optional — georgia, times, helvetica, arial, or any CSS font
  style: italic       # optional — CSS font-style
  size: large         # optional — small / medium / large / xlarge or any CSS value

short:
  text: One-liner shown on the slide.
  fg: 0xffffff        # optional — inherits title fg
  font: times         # optional — inherits title font
  size: medium        # optional — same keywords as title size

image:
  path: ./image.webp  # or plain string form: image: ./image.webp
  size: cover         # optional — cover (default), contain, or any CSS value
  position: center    # optional — any CSS background-position value

url: https://github.com/you/myproject
order: 1              # optional — lower = earlier; unset repos sort last then alphabetically
theme: dark           # optional — dark (default) or light; controls arrow/dot contrast

description:
  text: ./description.md   # path to description file
  fg: 0x222222             # required
  bg: 0xf0f0f2             # required
```

Colors use `0xRRGGBB` or `0xRRGGBBAA` notation (no quotes needed). `#RRGGBB` also
works but requires quotes in YAML since `#` starts a comment.

## build

```sh
go run ./cmd/generate
```

Output goes to `site/`. Preview locally:

```sh
python3 -m http.server 8080 --directory site
```

## deploy

Upload the `site/` directory to any static host — Netlify, GitHub Pages, Vercel, [surge](https://surge.sh), etc.
