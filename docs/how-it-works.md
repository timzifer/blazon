# How it works

Three stages. Each one is a seam you can cut at.

## 1 — Entropy

Not `sha256("1.4.2")` in one piece. Four separated streams:

```
h0 = hash(major)                      archetype
h1 = hash(major, minor)               variant
h2 = hash(major, minor, patch)        detail
h3 = hash(full version)               prerelease
```

Fields are length-prefixed, so `1.23.4` and `12.3.4` cannot collide.

## 2 — Parameter mapping

A `Renderer` draws its parameters from those streams and decides what the mark
*means*. This is the stage that determines whether the library is useful. Not
every renderer hashes: `cistercian`, `dial` and `orbit` encode the number
itself, so the version can be read back out of the mark.

## 3 — Render primitives

A `canvas.Canvas` turns drawing calls into SVG, pixels or terminal cells. This
stage decides only how it looks. Every renderer works on every back end.

## Policy: how related should neighbours look?

An API choice, not a design decision:

| Policy | Effect |
| --- | --- |
| `PolicyFamilyAmplified` (default) | Major sets the archetype, minor the variant, patch the detail — and each renderer routes two or three of its highest-contrast parameters through the patch stream, so adjacent patches stay in the family but are never hard to tell apart. |
| `PolicyFamily` | The plain cascade. Patch changes only fine detail. |
| `PolicyIndependent` | Every stream comes from the full version. Neighbours look unrelated: maximum distinguishability, no family information. |

A prerelease never changes the core streams: `1.0.0-rc.1` still reads as a
`1.0.0`, marked by a small band of dots.

## The testable invariant

Distinguishability is a test, not a claim. This is the part that separates
blazon from any identicon port.

Marks are rendered across a corpus of 184 versions, reduced to a 128-bit
perceptual hash — a DCT hash plus a difference hash, because either alone
confuses shapes the other separates — and the Hamming distances between related
versions are held to per-renderer thresholds:

```go
blazontest.Suite{
    Renderer:   myRenderer,
    Thresholds: blazontest.Thresholds{MinPatch: 6, MinMinor: 9, MinMajor: 20, Floor: 2},
}.Run(t)
```

The suite checks:

- **patch, minor and major neighbours** clear their minimum distances;
- **no pair anywhere** in the corpus falls below the global floor;
- **family cohesion** — versions sharing a major really are closer to each
  other than to other majors, so the family policy is not a lie;
- **prereleases stay close** to their release, bounded from above rather than
  below;
- **uniqueness** — no two versions render to the same bytes, exactly rather
  than perceptually;
- **determinism** — the same version renders identically, byte for byte where a
  renderer declares it can.

Measurement is monochrome on purpose. A luminance-based hash scores two marks
differing only in hue as identical, so measuring in colour would credit colour
for distinctness the form does not have. Judging form alone makes the guarantee
hold in print, in a monochrome terminal, and for a colour-blind reader.

Thresholds are calibrated from each renderer's own histogram, not guessed:

```
blazon dist -r truchet
```

## Output

- **SVG** — plain string building, deterministic number formatting.
- **PNG** — anti-aliased by exact area coverage: each edge deposits the signed
  area it sweeps per pixel column, and a running sum along the row gives
  coverage. Filling a shape and summing the alpha returns its geometric area,
  which is how the anti-aliasing is tested.
- **Terminal** — half-block characters give each cell two square pixels, so
  vector renderers arrive as pictures rather than as ASCII art. Falls back to a
  density ramp under `NO_COLOR`.

Colour is derived in OKLCH with gamut mapping, held in a lightness band that
keeps contrast on both a white page and a dark terminal.

## No dependencies

Standard library only — including the rasteriser and the font. Those are the two
things that decide what the pixels are, and a library whose whole premise is
that a version always produces the same mark cannot outsource them: a dependency
that sharpened its anti-aliasing would change every mark ever generated,
arriving as a routine version bump. A test enforces it.
