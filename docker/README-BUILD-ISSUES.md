# Docker Build Issues and Solutions

This document describes issues that may occur when building the debos Docker image in certain environments, particularly in CI/CD systems with MITM proxies or network restrictions.

## Issue 1: Certificate Errors with MITM Proxies

### Symptom
```
x509: certificate signed by unknown authority
```

When downloading Go dependencies from `proxy.golang.org` during the Docker build.

### Root Cause
Some environments (like GitHub Actions) use MITM proxies (e.g., GoProxy) that intercept HTTPS traffic with self-signed certificates from development CAs (e.g., mkcert). The Docker container doesn't trust these CA certificates by default.

### Solution Implemented
The Dockerfile now conditionally includes the mkcert CA certificate if present:

1. Copy your environment's CA certificate to `docker/mkcert-ca.crt`
2. The Dockerfile will automatically detect and add it to the container's trust store
3. This happens before the `go install` command, allowing Go to trust the proxy

**Example for GitHub Actions:**
```bash
# Copy the mkcert CA certificate if it exists
cp /home/runner/work/_temp/runtime-logs/mkcert/rootCA.pem docker/mkcert-ca.crt

# Build normally
docker build --network=host -t debos -f docker/Dockerfile .
```

### Verification
To check if your environment uses a MITM proxy:

```bash
# Check the certificate chain for proxy.golang.org
openssl s_client -connect proxy.golang.org:443 -showcerts </dev/null 2>&1 | grep -A 2 "subject="

# Look for self-signed CAs like:
# subject=O = mkcert development CA, OU = ...
```

## Issue 2: ArchLinux Keyring Download Failures

### Symptom
```
gzip: stdin: unexpected end of file
tar: Child returned status 1
```

When downloading the archlinux-keyring from `gitlab.archlinux.org`.

### Root Cause
- DNS resolution failure for `gitlab.archlinux.org`
- Network restrictions preventing access to external GitLab instances
- Incomplete downloads due to proxy issues

### Solution Implemented
The `get-archlinux-keyring.sh` script now handles failures gracefully:

1. Always creates the target directory even on failure
2. Creates a `.download-failed` marker file when download fails
3. Allows the Docker build to continue using Debian's packaged archlinux-keyring
4. The Dockerfile uses `|| true` to not fail the build on this step

**Note:** The Debian-packaged archlinux-keyring may be outdated, which could affect pacstrap actions. This is acceptable for most testing scenarios.

## Issue 3: General Network Restrictions

### Symptom
Various network-related errors during Docker build.

### Solution
Use `--network=host` flag when building:

```bash
docker build --network=host -t debos -f docker/Dockerfile .
```

This allows the Docker build to use the host's network configuration, which may have better access to required resources.

## Testing the Build

After successfully building the Docker image, verify it works:

```bash
# Check the image was created
docker images debos

# Test debos version
docker run --rm debos --version

# Run a simple integration test
cd tests
docker run --rm --device /dev/kvm \
  -v $(pwd):/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v recipes/test.yaml
```

## Environment-Specific Notes

### GitHub Actions
- Uses GoProxy MITM proxy with mkcert CA
- gitlab.archlinux.org is often unreachable due to DNS restrictions
- Both workarounds in this document are necessary

### Corporate Networks
- May have corporate CA certificates for HTTPS inspection
- Follow the same pattern: copy your corporate CA to `docker/mkcert-ca.crt`
- Ensure the certificate is in PEM format with `.crt` extension

### Local Development
- Usually doesn't require these workarounds
- If you see certificate errors, check for local proxies (Charles, Fiddler, etc.)
- The build should work without modifications in most local environments

## Updating the CA Certificate

The `docker/mkcert-ca.crt` file is:
- Gitignored (environment-specific)
- Optional (only needed in environments with MITM proxies)
- Automatically detected by the Dockerfile

To update it:
```bash
# Find your CA certificate location
find /home/runner -name "rootCA.pem" 2>/dev/null

# Copy it to the docker directory
cp <path-to-ca>/rootCA.pem docker/mkcert-ca.crt

# Rebuild
docker build --network=host -t debos -f docker/Dockerfile .
```

## Troubleshooting

### Build still fails with certificate errors
1. Verify the CA certificate is correct: `openssl x509 -in docker/mkcert-ca.crt -noout -subject`
2. Check if certificate is in PEM format
3. Ensure the certificate matches the one signing the MITM proxy certificates

### Build fails with "not found" error for /arch-keyring
1. Check if `get-archlinux-keyring.sh` is being executed
2. Verify the script has execute permissions
3. The script should create the directory even on failure - check script modifications

### Tests fail with KVM errors
1. Ensure `/dev/kvm` is accessible: `ls -la /dev/kvm`
2. Check user permissions: `groups | grep kvm`
3. Add `--privileged` flag to docker run if needed

## References

- Original issue: Docker build failing in GitHub Actions
- CA certificate location (GHA): `/home/runner/work/_temp/runtime-logs/mkcert/rootCA.pem`
- GoProxy documentation: https://github.com/goproxy/goproxy
- mkcert documentation: https://github.com/FiloSottile/mkcert
