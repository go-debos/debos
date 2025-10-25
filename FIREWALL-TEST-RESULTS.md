# Docker Build Firewall Test Results

## Test Setup
Branch reset to original version (commit a816568) to identify URLs requiring firewall allowlist access.

## Test Attempts

### Attempt 1 - Initial Test
**Date**: First test with firewall re-enabled  
**Status**: ❌ FAILED  
**Error**: Certificate validation failures

### Attempt 2 - After Allowlist Fix
**Date**: After proxy.golang.org properly added to allowlist  
**Status**: ❌ FAILED  
**Error**: Same certificate validation failures persist

## Build Result
**Status**: ❌ FAILED (both attempts)

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
Even with `proxy.golang.org` on the firewall allowlist, certificate validation still fails. This confirms:

**The firewall is performing HTTPS interception (MITM) on proxy.golang.org connections, presenting a self-signed certificate that the Go HTTP client doesn't trust.**

### Key Finding
**Adding proxy.golang.org to the allowlist does not solve the issue because the firewall is still performing HTTPS inspection/interception on the connection.**

The allowlist controls whether traffic is blocked, but the MITM proxy is still intercepting HTTPS connections and re-signing them with an untrusted certificate.

## Solutions

Four options to resolve the certificate validation issue:

### Option 1: Use Host CA Certificates (Recommended)
Expose the host's certificate store to the Docker build environment:
```bash
DOCKER_BUILDKIT=1 docker build --network=host \
  --secret id=cacert,src=/etc/ssl/certs/ca-certificates.crt \
  -t debos -f docker/Dockerfile .
```

**Pros**: 
- Allows use of Go module proxy with caching benefits
- Automatically trusts the MITM proxy's certificate
- Works with any firewall configuration
- No infrastructure changes needed

**Cons**: 
- Requires Docker BuildKit (enabled by default in recent Docker versions)
- Slightly more complex build command

### Option 2: Use GOPROXY=direct
Bypass proxy.golang.org entirely by setting `GOPROXY=direct`:
```bash
docker build --network=host --build-arg GOPROXY=direct -t debos -f docker/Dockerfile .
```

**Pros**: 
- Simple, no infrastructure changes needed
- Previously tested successfully when firewall was fully disabled
- Works with current firewall configuration

**Cons**: 
- Bypasses Go module proxy caching
- Direct connections to module sources required

### Option 3: Disable HTTPS Interception for proxy.golang.org
Configure the firewall to allow proxy.golang.org **without performing SSL/TLS inspection**.

**Pros**: 
- Allows use of Go module proxy with caching benefits
- No Docker build changes needed

**Cons**: 
- Requires firewall configuration changes
- May require separate policy for proxy.golang.org

### Option 4: Install MITM CA Certificate Permanently
Add the MITM proxy's CA certificate to Docker build environment permanently.

**Pros**: 
- Allows HTTPS inspection to continue for security monitoring

**Cons**: 
- Most complex solution
- Requires managing environment-specific certificates
- Certificate must be added to Docker build process
- Less flexible than Option 1

## Recommendation
Use **Option 1 (Host CA certificates)** as it provides the best balance:
- Works automatically with any MITM proxy
- Maintains Go proxy caching benefits
- No infrastructure or permanent code changes needed
- Simple to use in CI environments

Alternatively, use **Option 2 (GOPROXY=direct)** for simplicity if proxy caching is not important.

**The issue is not about allowlisting URLs - it's about how the firewall handles HTTPS traffic to proxy.golang.org.**
