# Contributing

Thanks for looking. This is a small library with one unusual rule, so the short
version first: **a change that makes version marks harder to tell apart is a
regression, and the build says so.**

## Getting set up

```
git clone https://github.com/timzifer/blazon
cd blazon
go test ./...
```

Go 1.21 or newer. CI builds against both that floor and the current release,
and fails if the two disagree with `go.mod`.

The only dependency is `golang.org/x/image`, used for rasterising and for the
bitmap font in contact sheets. It is pinned to v0.24.0 deliberately: later
releases require Go 1.23 and then 1.25, and taking them would raise this
library's floor for no benefit — v0.24.0 rasterises byte-identically to the
current release for everything drawn here. Dependabot is configured not to
offer those bumps.

Before opening a pull request:

```
gofmt -l .        # must print nothing
go vet ./...
go test -race ./...
```

## The rule that is not obvious

Every renderer has an entry in the threshold table in `distinctness_test.go`:

```go
"truchet": {
    MinPatch:        13,
    MinMinor:        28,
    MinMajor:        30,
    Floor:           12,
    PrereleaseRatio: 0.35,
},
```

Those are minimum perceptual distances, in bits of a 128-bit hash, between
versions that differ in exactly one component. They are **frozen measurements,
not aspirations**. If your change pushes any relation below its bound, the test
fails — and the right response is usually to fix the renderer, not to lower the
number.

Lowering a threshold is sometimes correct. When it is, say so in the pull
request and give the numbers: what the histogram looked like before, what it
looks like now, and why the new value is still a useful bound.

## Adding a renderer

1. Write it in a `render_<name>.go` file at the repository root, implementing
   `blazon.Renderer` and registering itself from `init`.

2. Draw parameters from the right streams. This is the whole design, so it is
   worth being deliberate:

   - `Params.Archetype()` — the coarse gestalt. Major only. A viewer should be
     able to name what changed here: a symmetry count, a shape family, a
     fingerprint pattern class.
   - `Params.Variant()` — major and minor. Variation within the gestalt.
   - `Params.Detail()` — the full core version. Changes on every patch.
   - `Params.Amplified()` — your two or three highest-contrast parameters.
     Under the default policy this follows the patch; under `PolicyFamily` it
     follows the minor. Draw from it rather than branching on the policy
     yourself.

   Rotation alone is rarely enough for an amplified parameter. A shape with
   *m*-fold symmetry barely changes when rotated, and the measurement will tell
   you so. Prefer something the low frequencies of the image can see: ridge
   spacing, ring count, grid size, an exponent that changes the outline.

3. Call `MarkPrerelease(ctx)` at the end, or `MarkPrereleaseCells` for a
   character-grid renderer. Without it, `1.0.0` and `1.0.0-rc.1` render to
   identical bytes, and two versions sharing one mark is the single failure
   this library may not have.

4. Declare your `Caps` honestly:

   - `ByteExact` only if every coordinate comes from integer arithmetic.
     `math.Sin` and `math.Pow` may differ by an ulp between architectures, so a
     renderer that uses them cannot promise this — and the CI matrix runs on
     three operating systems specifically to check.
   - `Ordered` if the mark encodes the version legibly rather than hashing it,
     as `cistercian` does. Ordered renderers are exempt from the distance
     thresholds — being systematic is the point — and are held to uniqueness
     instead.
   - `TextOnly` with a `GridCols`/`GridRows` if the renderer draws through
     `Canvas.Cell`.

5. Calibrate. Run

   ```
   go run ./cmd/blazon dist -r <name>
   ```

   It prints the distance histogram, the closest pairs, and a suggested set of
   thresholds at about 85% of each observed minimum. Take those as a starting
   point, sanity-check them against a contact sheet, and add the entry to
   `distinctness_test.go`.

6. Look at it.

   ```
   go run ./cmd/blazon sheet -r <name> -o sheet.png
   ```

   Numbers say whether a renderer passes. A contact sheet is how you find out
   whether it is any good.

7. Regenerate the gallery in the same pull request:

   ```
   go run ./cmd/blazon gallery -o docs/gallery -readme README.md
   ```

   Run it twice. The second run must produce no diff. CI regenerates the
   gallery on every pull request and fails if the committed images do not
   match, so a stale gallery is a red build rather than a silent drift.

   **Regenerate with the current Go release, not the 1.21 floor.** The
   comparison is byte for byte, and the Go compiler is allowed to contract
   float expressions differently between versions: the same code on Go 1.21 and
   on Go 1.27 produces visually identical marks whose pixels differ slightly.
   CI pins the gallery job to the current release for exactly this reason. It
   is also why the library's own determinism check compares float renderers by
   perceptual hash rather than by bytes — see `Caps.ByteExact`.

## Changing an existing renderer

Any change to what a renderer draws changes the gallery, and the committed
images are the visual regression baseline. Regenerate them, and look at the
image diff before you push: that diff is the review. CI checks that you did.

If the change moves a threshold, re-run `dist` and update the table with the
new numbers rather than nudging the old ones until the test goes green.

## Commit messages and pull requests

Conventional Commits (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`). The
subject line says what changed; the body says why, especially when the *why* is
a measurement.

Pull requests go to `main` and need a green CI run. The `distinctness` job is
the gate.

## Releasing

Maintainers only, and it is two steps because the library signs its own
releases with its own marks:

1. Bump `blazon.Release` in `blazon.go` and regenerate the gallery — the logo
   at the top of the README is the flowfield mark for that exact version.
2. Tag `v<the same version>`. The release workflow refuses a tag that does not
   match the constant.

## Scope

Things that fit: new renderers, new output back ends, better calibration tools,
sharper measurements.

Things that probably do not: rendering anything that is not a version number,
and configuration options that make the marks for a given version depend on
something other than that version. Determinism is not negotiable — the same
version has to produce the same mark, everywhere, forever.
