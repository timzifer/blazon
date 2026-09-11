# Hosted marks

If all you want is a mark in a README or a web UI, there is an endpoint for it —
no build step, no asset pipeline:

```
https://api.blazon.build/v1/mark/1.4.2.svg
https://api.blazon.build/v1/mark/1.4.2.svg?renderer=flowfield&bg=dark
https://api.blazon.build/v1/mark/1.4.2.png?renderer=bishop&size=128
```

```html
<img src="https://api.blazon.build/v1/mark/1.4.2.svg" width="24" alt="v1.4.2">
```

`/v1/mark/{version}.{svg|png|txt}`, with `renderer`, `size`, `padding`,
`policy`, `palette`, `bg`, `cols` and `nocolor` spelled exactly as the CLI
spells them. `/v1/renderers` lists what exists and the current size ceilings;
`/v1/healthz` reports which release is deployed. A mark is a pure function of
its URL, so the answers are cacheable forever and are served that way.

It runs the same code this repository publishes — the deploy compares every
response against a `go run` of the same version, byte for byte, and refuses to
ship on a disagreement. So the marks are the marks. What differs is the budget.

## Limits

`size` runs from 16 up, with a ceiling per format: **1024 for SVG, 128 for
PNG**. The difference is not arbitrary. SVG emits geometry, and its cost is
whatever the renderer's own maths costs — the requested size only ever reaches
the `viewBox` attribute. PNG rasterises and deflates, so its cost grows with the
square of the edge length.

That is a limit of one small deployment, not of the library. `blazon.PNG` has no
ceiling, no renderer is excluded, and rendering locally costs a few
milliseconds:

```go
data, err := blazon.PNG("1.4.2", blazon.Options{Renderer: "flowfield", Size: 1024})
```

Fetching a mark that has already been drawn is not rate limited; drawing a new
one is, per address. Asking for a few hundred distinct marks in a burst will meet
a `429`, and the answer to that is a `for` loop against the library rather than
against the endpoint.

The service is best effort and carries no availability promise. Anything that
matters should render its own marks — that is the whole point of a library with
no dependencies.
