<p align="center">
  <img src="docs/logo.svg" alt="blazon's own mark for its current release" width="150">
</p>

<h1 align="center">blazon</h1>

<p align="center">
  <em>The mark above is blazon's own version, drawn by blazon.<br>
  It changes with every release.</em>
</p>

<p align="center">
  <a href="https://github.com/timzifer/blazon/actions/workflows/ci.yml"><img src="https://github.com/timzifer/blazon/actions/workflows/ci.yml/badge.svg" alt="ci"></a>
  <a href="https://pkg.go.dev/github.com/timzifer/blazon"><img src="https://pkg.go.dev/badge/github.com/timzifer/blazon.svg" alt="go reference"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT"></a>
</p>

**Give every version of your software a face.** blazon turns a version number
into a small picture that a person recognises at a glance, without reading it.

## Why

Version numbers are hard to see. `1.4.12` and `1.4.2` differ by one character in
the middle of a string, and every mistake that follows from mixing them up looks
like a different bug. A picture fails differently: two versions either look the
same or they do not, and the eye answers in one glance.

- **Spot the odd one in a list.** A release table, a deploy dashboard, a fleet of
  agents, a row of build artifacts — one mark per row and the outlier stops
  hiding among digits.
- **Show the version without showing a version.** A mark in a corner, a favicon,
  a splash screen, a login footer: users never see a number, but a screenshot in
  a bug report still tells you which build it came from, and two screenshots
  from "the same version" stop being a guess.
- **Confirm what shipped.** The mark on the artifact, on the running service's
  health page and in the release notes either match or they do not. No diffing
  strings across three tabs.
- **Make releases memorable.** A changelog entry, a tag, a launch post — the mark
  is what people remember the release *as*.

Two properties make that work, and both are enforced by tests rather than
claimed:

1. **Neighbours are never confusable.** A patch bump visibly changes the mark. A
   change that makes patch bumps harder to tell apart turns the build red.
2. **Family resemblance is deliberate.** The version's structure is the input —
   major, minor and patch feed separate entropy streams — so all `1.x` marks read
   as relatives while `2.0.0` starts over. How closely neighbours resemble each
   other is [a setting](docs/how-it-works.md#policy-how-related-should-neighbours-look),
   not an accident of a hash.

And if you would rather a person be able to *read* the version back out of the
mark, three of the renderers encode the number itself instead of hashing it.

## Use it

Hosted, for a README or a web UI — no build step:

```html
<img src="https://api.blazon.build/v1/mark/1.4.2.svg" width="24" alt="v1.4.2">
```

In Go:

```go
import "github.com/timzifer/blazon"

svg, err := blazon.SVG("1.4.2", blazon.Options{})
png, err := blazon.PNG("1.4.2", blazon.Options{Renderer: "flowfield", Size: 512})
txt, err := blazon.Text("1.4.2", blazon.Options{Renderer: "bishop"})
```

From a terminal or a build script:

```
go install github.com/timzifer/blazon/cmd/blazon@latest

blazon 1.4.2                        # draw it in the terminal
blazon 1.4.2 -r flowfield -o x.svg
```

**No dependencies** — standard library only, including the rasteriser and the
font. A library whose premise is that a version always produces the same mark
cannot outsource the code that decides what the pixels are.
[Why that matters](docs/how-it-works.md#no-dependencies).

## Docs

- **[Renderers and gallery](docs/renderers.md)** — the eight marks, and what
  every version looks like in each.
- **[How it works](docs/how-it-works.md)** — entropy streams, family policy, the
  output back ends, and the distinctness test suite.
- **[Hosted service](docs/service.md)** — the endpoint, its parameters and its
  limits.
- **[Contributing](CONTRIBUTING.md)** — a new renderer needs an entry in the
  threshold table and has to pass `blazontest.Suite`.

## License

MIT.
