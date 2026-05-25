# tivoli

![CI](https://github.com/awkward-crow/tivoli/actions/workflows/ci.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/awkward-crow/tivoli)](https://goreportcard.com/report/github.com/awkward-crow/tivoli)

A Go static site generator that produces a single-page carousel for showcasing
GitHub repos. Works on mobile and desktop. No npm, no framework.

**Demo:** [awkward-crow.surge.sh](https://awkward-crow.surge.sh)

## what's here

```
cmd/tivoli/main.go                    generator
cmd/tivoli/templates/index.html.tmpl  page template
cmd/tivoli/static/style.css           CSS scroll-snap carousel
cmd/tivoli/static/carousel.js         nav arrows, dots, keyboard
```

## adding a repo to the carousel

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

## upstream repos

A repo on another site can advertise its own slide by adding a `.tivoli/`
config directory to its root:

```
.tivoli/
  config.yaml       full slide config (same format as above)
  description.md    optional — path is relative to .tivoli/
  image.webp        optional background image
```

To include such a repo in your carousel, add a stub entry under `repos/` with
only `url` set (no `title`):

```
repos/myproject/
  config.yaml       stub — url only
```

```yaml
url: https://github.com/you/myproject
order: 2            # optional — overrides the upstream order
```

Tivoli does a shallow sparse clone of the upstream repo, reads
`.tivoli/config.yaml`, and uses it as the full slide config. The `url` in the
stub is always authoritative; `order` in the stub overrides the upstream value
if set.

## build and run

From inside this repo:

```sh
go run ./cmd/tivoli
```

Or build and install the binary once, then run it from anywhere:

```sh
go install ./cmd/tivoli   # installs to ~/go/bin/tivoli by default
tivoli -repos ~/my-site/repos -out ~/my-site/site
```

The directories default to `repos/` and `site/` relative to the working directory if no
flags are given. Output goes to the `site/` directory. Preview locally:

```sh
python3 -m http.server 8080 --directory site
```

## deploy

Upload the `site/` directory to any static host — Netlify, GitHub Pages, Vercel, [surge](https://surge.sh), etc.
