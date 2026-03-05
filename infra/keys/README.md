# Signing Keys

This directory holds cosign key material **locally only**. Key files are
gitignored — never commit `*.key` or `*.pub` files to the repository.

## Generating a Key Pair

```bash
# Install cosign if needed
brew install cosign

# Generate an encrypted ECDSA key pair.
# You will be prompted for a passphrase; use a strong one for production.
# For local development, an empty passphrase is acceptable.
cosign generate-key-pair

# Move the generated files into this directory.
mv cosign.key infra/keys/cosign-dev.key
mv cosign.pub infra/keys/cosign-dev.pub
```

## Storing Keys in GitHub Actions Secrets

The pipelines expect the following repository secrets. Set them via the
GitHub UI (`Settings → Secrets and variables → Actions → New repository secret`)
or with the GitHub CLI:

```bash
# Private key (PEM-encoded, may be encrypted with a passphrase)
gh secret set COSIGN_PRIVATE_KEY < infra/keys/cosign-dev.key

# Passphrase used to decrypt the private key (empty string for dev keys)
gh secret set COSIGN_PASSWORD --body ""

# Public key (used by the CLI verifier and the API's /v1/signing-keys/current)
gh secret set COSIGN_PUBLIC_KEY < infra/keys/cosign-dev.pub
```

For production, rotate to a key with a strong passphrase and store it in
your secrets manager (e.g. AWS Secrets Manager, HashiCorp Vault) rather than
directly in GitHub secrets.

## How the Keys Are Used

| Component | Key | How |
|---|---|---|
| `pipeline/signing` | Private key | Signs bottles and SBOMs via cosign SDK |
| `cli/internal/verifier` | Public key | Verifies signatures before install |
| `POST /v1/signing-keys/current` | Public key | Delivers the public key to the CLI |
| `post-approval-pipeline.yml` | Private key | Signs external-binary bottles before publishing |

## Key Rotation

1. Generate a new key pair with `cosign generate-key-pair`
2. Update the GitHub secrets with the new keys
3. Insert a new row in the `signing_keys` DB table with `active = true` and
   update the old row to `active = false`
4. Old signatures remain valid — the verifier only needs the public key
   that was active at signing time, which is stored in `signing_keys`
