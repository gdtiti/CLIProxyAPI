# Docker Branch Build Guide

## Overview

This guide explains how to build and use Docker images from development branches, including the `CLIProxyAPIPlus-gdtiti` branch with the Gemini CLI "ALL" selection fix.

## Automated Branch Builds

### Supported Branches

The GitHub Actions workflow automatically builds Docker images for:
- `CLIProxyAPIPlus-gdtiti` - Current development branch with Gemini CLI fix
- `feature/**` - Feature branches
- `fix/**` - Bug fix branches
- `dev/**` - Development branches

### Trigger Methods

#### 1. Automatic Build on Push

When you push to any supported branch, GitHub Actions automatically builds and publishes Docker images:

```bash
git push origin CLIProxyAPIPlus-gdtiti
```

#### 2. Manual Workflow Dispatch

You can manually trigger a build from GitHub:

1. Go to **Actions** tab in GitHub repository
2. Select **Docker Branch Build** workflow
3. Click **Run workflow**
4. (Optional) Specify a branch name, or leave empty for current branch
5. Click **Run workflow** button

### Image Tags

Each branch build creates multiple tags for flexibility:

| Tag Format | Example | Description |
|------------|---------|-------------|
| `{branch}` | `cliproxyapiplus-gdtiti` | Latest build from branch |
| `{branch}-{commit}` | `cliproxyapiplus-gdtiti-a1b2c3d` | Specific commit |
| `{branch}-{timestamp}` | `cliproxyapiplus-gdtiti-20231231-120000` | Timestamped build |

**Note**: Branch names are sanitized for Docker compatibility (lowercase, special chars replaced with `-`)

## Using Branch Images

### Pull the Latest Branch Image

```bash
# Pull latest from CLIProxyAPIPlus-gdtiti branch
docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti

# Pull specific commit
docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti-a1b2c3d

# Pull timestamped version
docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti-20231231-120000
```

### Run the Branch Image

#### Basic Usage

```bash
docker run -d \
  --name cliproxyapi-dev \
  -p 8317:8317 \
  -v $(pwd)/config.yaml:/CLIProxyAPI/config.yaml \
  -v ~/.gemini:/root/.gemini \
  -v ~/.codex:/root/.codex \
  eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti
```

#### With Docker Compose

Create `docker-compose.dev.yml`:

```yaml
version: '3.8'

services:
  cliproxyapi-dev:
    image: eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti
    container_name: cliproxyapi-dev
    ports:
      - "8317:8317"
    volumes:
      - ./config.yaml:/CLIProxyAPI/config.yaml
      - ~/.gemini:/root/.gemini
      - ~/.codex:/root/.codex
      - ~/.kiro:/root/.kiro
    environment:
      - TZ=Asia/Shanghai
    restart: unless-stopped
```

Run with:

```bash
docker-compose -f docker-compose.dev.yml up -d
```

### Testing the Gemini CLI Fix

The `CLIProxyAPIPlus-gdtiti` branch includes the fix for Gemini CLI "ALL" selection. To test:

1. **Start the container**:
   ```bash
   docker run -d \
     --name cliproxyapi-test \
     -p 8317:8317 \
     -v ~/.gemini:/root/.gemini \
     eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti
   ```

2. **Login with Gemini CLI** (inside container):
   ```bash
   docker exec -it cliproxyapi-test sh
   # Inside container
   gemini-cli login
   # Select "ALL" when prompted
   ```

3. **Verify the fix**:
   - Check that credentials are saved even if some projects fail
   - Look for warning messages about skipped projects
   - Verify credential file contains successful projects

4. **Check logs**:
   ```bash
   docker logs cliproxyapi-test
   ```

## Build Information

Each branch image includes build metadata:

```bash
# Check version info
docker run --rm eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti ./CLIProxyAPIPlus --version

# Inspect image labels
docker inspect eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti | jq '.[0].Config.Labels'
```

## Comparing with Production

### Production Image (Tagged Releases)

```bash
# Latest stable release
docker pull eceasy/cli-proxy-api-plus:latest

# Specific version
docker pull eceasy/cli-proxy-api-plus:v1.2.3
```

### Development Image (Branch)

```bash
# Latest development build with Gemini fix
docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti
```

### Side-by-Side Testing

Run both versions simultaneously:

```bash
# Production on port 8317
docker run -d \
  --name cliproxyapi-prod \
  -p 8317:8317 \
  eceasy/cli-proxy-api-plus:latest

# Development on port 8318
docker run -d \
  --name cliproxyapi-dev \
  -p 8318:8317 \
  eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti
```

## Troubleshooting

### Image Not Found

If the image is not available:

1. **Check GitHub Actions**: Go to Actions tab and verify the build completed successfully
2. **Check DockerHub**: Visit https://hub.docker.com/r/eceasy/cli-proxy-api-plus/tags
3. **Trigger manual build**: Use workflow dispatch to rebuild

### Build Failures

Common issues:

1. **Missing secrets**: Ensure `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` are configured in repository secrets
2. **Dockerfile errors**: Check the Dockerfile is valid for the branch
3. **Build timeout**: Large builds may timeout; check GitHub Actions logs

### Authentication Issues

If you can't pull the image:

```bash
# Login to DockerHub
docker login

# Then pull
docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti
```

## CI/CD Workflow Details

### Workflow File

Location: `.github/workflows/docker-branch.yml`

### Build Process

1. **Checkout**: Clones the specified branch
2. **Setup**: Configures QEMU and Docker Buildx for multi-platform builds
3. **Login**: Authenticates with DockerHub
4. **Metadata**: Generates version, commit, and timestamp information
5. **Build**: Builds for `linux/amd64` and `linux/arm64`
6. **Push**: Publishes images with multiple tags
7. **Cache**: Uses GitHub Actions cache for faster subsequent builds

### Build Platforms

- `linux/amd64` - x86_64 architecture (most common)
- `linux/arm64` - ARM64 architecture (Apple Silicon, ARM servers)

### Build Arguments

The following build arguments are injected:

- `VERSION`: Git describe + branch name
- `COMMIT`: Short commit hash
- `BUILD_DATE`: ISO 8601 timestamp

## Best Practices

### For Developers

1. **Use specific tags for testing**:
   ```bash
   # Good - specific commit
   docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti-a1b2c3d

   # Risky - may change
   docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti
   ```

2. **Tag your test deployments**:
   ```bash
   docker tag eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti my-test:v1
   ```

3. **Clean up old images**:
   ```bash
   docker image prune -a
   ```

### For CI/CD

1. **Pin to commit-specific tags** in production-like environments
2. **Use branch tags** for development/staging environments
3. **Monitor image sizes**: Branch builds should be similar to release builds

## Migration to Production

When the branch is ready for production:

1. **Merge to main**:
   ```bash
   git checkout main
   git merge CLIProxyAPIPlus-gdtiti
   git push origin main
   ```

2. **Create a release tag**:
   ```bash
   git tag -a v1.3.0 -m "Release v1.3.0 - Gemini CLI ALL fix"
   git push origin v1.3.0
   ```

3. **Production build triggers automatically** via `docker-image.yml` workflow

4. **Update deployments** to use the new version:
   ```bash
   docker pull eceasy/cli-proxy-api-plus:v1.3.0
   docker pull eceasy/cli-proxy-api-plus:latest
   ```

## Monitoring Builds

### GitHub Actions

View build status:
- Repository → Actions → Docker Branch Build
- Check logs for each step
- Download artifacts if available

### DockerHub

View published images:
- https://hub.docker.com/r/eceasy/cli-proxy-api-plus/tags
- Filter by branch name
- Check image sizes and platforms

### Notifications

Configure GitHub notifications:
- Settings → Notifications → Actions
- Enable email/Slack notifications for build failures

## Advanced Usage

### Multi-Stage Testing

```bash
# Stage 1: Pull and test branch image
docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti
docker run -d --name test1 -p 8317:8317 eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti

# Stage 2: Run integration tests
./run-integration-tests.sh http://localhost:8317

# Stage 3: Promote to staging
docker tag eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti my-registry/cliproxyapi:staging
docker push my-registry/cliproxyapi:staging
```

### Custom Builds

Build locally with same parameters:

```bash
# Get current commit
COMMIT=$(git rev-parse --short HEAD)
VERSION=$(git describe --tags --always --dirty)-cliproxyapiplus-gdtiti
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Build
docker build \
  --build-arg VERSION=$VERSION \
  --build-arg COMMIT=$COMMIT \
  --build-arg BUILD_DATE=$BUILD_DATE \
  -t my-local-build:test \
  .
```

## Security Considerations

1. **Secrets Management**: DockerHub credentials are stored as GitHub secrets
2. **Image Scanning**: Consider adding vulnerability scanning to the workflow
3. **Access Control**: Limit who can trigger manual builds
4. **Credential Volumes**: Be careful when mounting credential directories

## Related Documentation

- [Gemini CLI ALL Fix](./GEMINI_ALL_FIX.md) - Technical details of the fix in this branch
- [Troubleshooting Guide](./TROUBLESHOOTING.md) - Common issues and solutions
- [Development Scripts](./DEV_SCRIPTS.md) - Local development tools

## Support

For issues with Docker builds:
1. Check GitHub Actions logs
2. Review DockerHub repository
3. Create an issue with build logs
4. Contact maintainers

---

**Last Updated**: 2024-12-31
**Branch**: CLIProxyAPIPlus-gdtiti
**Workflow**: `.github/workflows/docker-branch.yml`
