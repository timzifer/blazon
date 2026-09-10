# Security policy

## Supported versions

blazon is pre-1.0. Only the latest release receives fixes.

| Version | Supported |
| --- | --- |
| 0.1.x | yes |

## Reporting a vulnerability

Report privately through GitHub:
[**Report a vulnerability**](https://github.com/timzifer/blazon/security/advisories/new).
Please do not open a public issue for a security problem.

Expect an acknowledgement within a week. If a report is accepted, the fix and
the advisory are published together.

## Threat model

blazon turns a version number into a picture. It is not a security primitive,
and nothing about it should be treated as one:

- **The marks are not authenticators.** They are derived from a public version
  string with a public algorithm. Anyone can produce the mark for any version.
  A mark proves nothing about provenance, integrity or identity.
- **Perceptual distance is not collision resistance.** The distinctness suite
  measures how easily a *human* tells two marks apart. It says nothing about an
  adversary deliberately searching for a version whose mark resembles another
  one's.
- **SHA-256 is used for reproducibility, not secrecy.** There is no secret
  input anywhere in the pipeline.

Given that, a genuine vulnerability here is most likely a memory-safety or
denial-of-service problem reachable from untrusted input — a version string, a
corpus file, or a renderer's parameters driving unbounded work. Those are worth
reporting. So is anything that breaks determinism in a way that could be
triggered remotely.
