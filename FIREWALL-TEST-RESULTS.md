# Docker Build Firewall Test Results

## Test Setup
Branch reset to original version (commit a816568) to identify URLs requiring firewall allowlist access.

## Build Result
**Status**: ❌ FAILED

**Error**: Certificate validation failures for all Go module downloads

## Certificate Errors
All downloads fail with:
```
x509: certificate signed by unknown authority
```

## URLs Accessed
All Go module downloads route through `proxy.golang.org`:

1. https://proxy.golang.org/al.essio.dev/pkg/shellescape/@v/v1.6.0.zip
2. https://proxy.golang.org/github.com/docker/go-units/@v/v0.5.0.zip
3. https://proxy.golang.org/github.com/freddierice/go-losetup/v2/@v/v2.0.1.zip
4. https://proxy.golang.org/github.com/go-debos/fakemachine/@v/v0.0.11.zip
5. https://proxy.golang.org/github.com/go-task/slim-sprig/v3/@v/v3.0.0.zip
6. https://proxy.golang.org/github.com/google/uuid/@v/v1.6.0.zip
7. https://proxy.golang.org/github.com/jessevdk/go-flags/@v/v1.6.1.zip
8. https://proxy.golang.org/github.com/sjoerdsimons/ostree-go/@v/v0.0.0-20201014091107-8fae757256f8.zip
9. https://proxy.golang.org/gopkg.in/yaml.v2/@v/v2.4.0.zip

## Analysis

### Root Cause
Since `proxy.golang.org` is already on the firewall allowlist but certificate validation still fails, this indicates:

**The firewall is performing HTTPS interception (MITM) with a self-signed certificate that the Go client doesn't trust.**

### Key Finding
**No additional URLs need to be added to the firewall allowlist.**

All Go module downloads route exclusively through `proxy.golang.org`. The issue is not missing URLs, but rather the MITM proxy's certificate not being trusted.

## Solutions

Three options to resolve the certificate validation issue:

### Option 1: Use GOPROXY=direct (Recommended)
Bypass proxy.golang.org entirely by setting `GOPROXY=direct`:
```bash
docker build --network=host --build-arg GOPROXY=direct -t debos -f docker/Dockerfile .
```

**Pros**: 
- Simple, no infrastructure changes needed
- Previously tested successfully when firewall was disabled

**Cons**: 
- Bypasses Go module proxy caching

### Option 2: Disable HTTPS Interception
Configure the firewall to allow proxy.golang.org without performing HTTPS interception.

**Pros**: 
- Allows use of Go module proxy
- No Docker build changes needed

**Cons**: 
- Requires firewall configuration changes

### Option 3: Install MITM CA Certificate
Add the MITM proxy's CA certificate to Docker build environment.

**Pros**: 
- Allows HTTPS inspection to continue

**Cons**: 
- Most complex solution
- Requires managing environment-specific certificates

## Recommendation
Use **Option 1 (GOPROXY=direct)** as it's the simplest solution that doesn't require infrastructure changes or certificate management.
