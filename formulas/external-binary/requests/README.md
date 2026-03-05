# External-Binary Package Requests

This directory contains YAML request files for third-party binary packages
that require manual approval before they can be installed by developers.

## How to Submit a Request

1. Create a new file named `{name}-{version}.yaml` in this directory.
2. Fill in all required fields (see schema below).
3. Open a pull request. The pipeline will automatically scan the binary and
   post a scan summary as a PR comment.
4. An admin reviews the scan results and approves or denies the request via
   the bark API.
5. On approval, the binary is signed and published. Developers can then
   `pkgtool install {name}@{version}`.

## Request Schema

```yaml
# Required fields
name: string           # Package name (lowercase, hyphens allowed)
version: string        # Version string (e.g. "1.2.3")
source_url: string     # Direct download URL for the binary tarball
sha256: string         # Expected hex-encoded SHA-256 digest of the tarball
requested_by: string   # GitHub handle or email of the requester
reason: string         # Business justification for adding this package
license_expected: string  # Expected SPDX license identifier (e.g. "MIT")
```

## Example

See [example.yaml](./example.yaml) for a complete example.

## Pipeline Behavior

The external-binary pipeline:
1. Downloads the binary from `source_url`
2. Verifies the SHA-256 against the `sha256` field
3. Runs `syft` to generate an SBOM
4. Runs `grype` to scan for CVEs
5. Runs `scancode` to detect licenses (non-blocking)
6. Posts a scan summary to the PR
7. Calls the bark API to create a `pending` version record

The package is **not installable** until an admin approves it.
