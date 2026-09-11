## What this changes

<!-- One or two sentences. What is different afterwards? -->

## Why

<!-- Especially when the "why" is a measurement, give the numbers. -->

## Checklist

- [ ] `gofmt -l .` prints nothing
- [ ] `go vet ./...` is clean
- [ ] `go test -race ./...` passes

If this changes what any renderer draws:

- [ ] Gallery regenerated (`go run ./cmd/blazon gallery -o docs/gallery -readme docs/renderers.md`),
      and a second run produced no diff
- [ ] I looked at the image diff

If this touches thresholds in `distinctness_test.go`:

- [ ] `go run ./cmd/blazon dist -r <renderer>` output is quoted below, and the
      new bounds are justified rather than nudged until the test went green

<!--
Lowering a threshold is sometimes right. When it is, say what the histogram
looked like before and after, and why the new value is still a useful bound.
-->
