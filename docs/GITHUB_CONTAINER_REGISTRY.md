# GitHub Container Registry Setup Guide

This guide explains how to use GitHub Container Registry (ghcr.io) instead of DockerHub for storing Docker images.

## Why GitHub Container Registry?

- ✅ **No External Secrets Required**: Uses built-in `GITHUB_TOKEN`
- ✅ **Integrated with GitHub**: Automatic permissions management
- ✅ **Free for Public Repos**: Unlimited storage and bandwidth
- ✅ **Better Integration**: Works seamlessly with GitHub Actions

## Configuration

### 1. Workflow Configuration

The workflow is already configured to use GitHub Container Registry:

```yaml
env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  docker-branch:
    permissions:
      contents: read
      packages: write  # Required for pushing to ghcr.io
```

### 2. Authentication

**No secrets needed!** The workflow uses the built-in `GITHUB_TOKEN`:

```yaml
- name: Login to GitHub Container Registry
  uses: docker/login-action@v3
  with:
    registry: ${{ env.REGISTRY }}
    username: ${{ github.actor }}
    password: ${{ secrets.GITHUB_TOKEN }}
```

### 3. Image Naming

Images are automatically named based on your repository:

```
ghcr.io/<owner>/<repository>:<tag>

Example:
ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti
```

## Using the Images

### Pull Images

**Public Repository** (no authentication needed):
```bash
docker pull ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti
```

**Private Repository** (authentication required):
```bash
# Create a Personal Access Token (PAT) with read:packages scope
# https://github.com/settings/tokens

# Login
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Pull
docker pull ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti
```

### Run Containers

```bash
docker run -d \
  --name cliproxyapi-dev \
  -p 8317:8317 \
  -v ~/.gemini:/root/.gemini \
  ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti
```

## Package Visibility

### Making Packages Public

By default, packages may be private. To make them public:

1. Go to your repository on GitHub
2. Click **Packages** (right sidebar)
3. Click on your package name
4. Click **Package settings**
5. Scroll to **Danger Zone**
6. Click **Change visibility**
7. Select **Public**
8. Confirm the change

### Package Permissions

GitHub automatically manages permissions:
- **Repository collaborators**: Can pull images
- **Public packages**: Anyone can pull
- **Workflow**: Can push with `packages: write` permission

## Available Tags

After pushing to the branch, images will be available with these tags:

```bash
# Latest from branch
ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti

# Specific commit
ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti-a1b2c3d

# Timestamped
ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti-20241231-120000
```

## Viewing Packages

### On GitHub

1. Go to your repository
2. Click **Packages** in the right sidebar
3. View all published packages and tags

### Via API

```bash
# List packages
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/orgs/YOUR_ORG/packages/container/cliproxyapiplus/versions

# Get package details
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/orgs/YOUR_ORG/packages/container/cliproxyapiplus
```

## Troubleshooting

### Error: Permission Denied

**Problem**: `Error: failed to push: denied: permission_denied`

**Solution**: Ensure the workflow has `packages: write` permission:
```yaml
jobs:
  docker-branch:
    permissions:
      contents: read
      packages: write  # Add this
```

### Error: Package Not Found

**Problem**: Can't pull the image after successful build

**Solutions**:
1. Check if package is public (see "Making Packages Public" above)
2. Verify the image name matches your repository
3. Check if you need authentication for private packages

### Error: Authentication Required

**Problem**: `Error: pull access denied`

**Solution**: Login to ghcr.io:
```bash
# Create PAT at https://github.com/settings/tokens
# Scopes needed: read:packages

echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin
```

### Image Not Appearing in Packages

**Problem**: Build succeeds but package doesn't show up

**Solutions**:
1. Wait a few minutes (can take time to appear)
2. Check workflow logs for push errors
3. Verify `push: true` in workflow
4. Check repository permissions

## Comparison: ghcr.io vs DockerHub

| Feature | GitHub Container Registry | DockerHub |
|---------|--------------------------|-----------|
| **Authentication** | Built-in `GITHUB_TOKEN` | Requires secrets |
| **Setup** | Zero configuration | Need to create secrets |
| **Cost** | Free for public repos | Free tier limited |
| **Integration** | Native GitHub integration | External service |
| **Permissions** | Automatic from repo | Manual management |
| **Visibility** | Linked to repository | Separate platform |

## Best Practices

### 1. Use Semantic Versioning

Tag releases with semantic versions:
```yaml
tags: |
  type=semver,pattern={{version}}
  type=semver,pattern={{major}}.{{minor}}
  type=semver,pattern={{major}}
```

### 2. Clean Up Old Images

Set up automatic cleanup:
```yaml
- name: Delete old packages
  uses: actions/delete-package-versions@v4
  with:
    package-name: cliproxyapiplus
    min-versions-to-keep: 10
```

### 3. Use Multi-Stage Builds

Optimize image size:
```dockerfile
FROM golang:1.24-alpine AS builder
# Build stage

FROM alpine:3.22.0
# Runtime stage
COPY --from=builder /app/binary /app/
```

### 4. Add Labels

Include metadata in images:
```yaml
labels: |
  org.opencontainers.image.source=${{ github.server_url }}/${{ github.repository }}
  org.opencontainers.image.revision=${{ github.sha }}
  org.opencontainers.image.created=${{ steps.meta.outputs.created }}
```

## Migration from DockerHub

If you were using DockerHub before:

### 1. Update Workflow

Already done! The workflow now uses ghcr.io.

### 2. Update Documentation

Update all references from:
```bash
docker pull eceasy/cli-proxy-api-plus:tag
```

To:
```bash
docker pull ghcr.io/your-org/cliproxyapiplus:tag
```

### 3. Update Deployment Scripts

Update any CI/CD or deployment scripts that reference the old image names.

### 4. Notify Users

Let users know about the new image location:
- Update README
- Create a GitHub release note
- Add a notice in the old DockerHub repository

## Advanced Usage

### Using in Docker Compose

```yaml
version: '3.8'

services:
  cliproxyapi:
    image: ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti
    ports:
      - "8317:8317"
    volumes:
      - ~/.gemini:/root/.gemini
    environment:
      - TZ=Asia/Shanghai
```

### Using in Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cliproxyapi
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cliproxyapi
  template:
    metadata:
      labels:
        app: cliproxyapi
    spec:
      containers:
      - name: cliproxyapi
        image: ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti
        ports:
        - containerPort: 8317
```

### Pulling in CI/CD

```yaml
# GitHub Actions
- name: Pull image
  run: docker pull ghcr.io/${{ github.repository }}:${{ github.sha }}

# GitLab CI
script:
  - echo $CI_REGISTRY_PASSWORD | docker login -u $CI_REGISTRY_USER --password-stdin ghcr.io
  - docker pull ghcr.io/your-org/cliproxyapiplus:latest
```

## Security Considerations

### 1. Token Permissions

The `GITHUB_TOKEN` has limited scope:
- ✅ Can push to packages in the same repository
- ✅ Automatically expires after workflow
- ❌ Cannot access other repositories
- ❌ Cannot modify repository settings

### 2. Package Scanning

Enable vulnerability scanning:
```yaml
- name: Run Trivy vulnerability scanner
  uses: aquasecurity/trivy-action@master
  with:
    image-ref: ghcr.io/${{ github.repository }}:${{ github.sha }}
    format: 'sarif'
    output: 'trivy-results.sarif'
```

### 3. Image Signing

Sign images for verification:
```yaml
- name: Sign image
  uses: sigstore/cosign-installer@main
- run: |
    cosign sign ghcr.io/${{ github.repository }}:${{ github.sha }}
```

## Monitoring and Maintenance

### Check Package Size

```bash
# Get package details
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/orgs/YOUR_ORG/packages/container/cliproxyapiplus \
  | jq '.size'
```

### List All Versions

```bash
# List all tags
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/orgs/YOUR_ORG/packages/container/cliproxyapiplus/versions \
  | jq '.[].metadata.container.tags'
```

### Delete Old Versions

```bash
# Delete specific version
curl -X DELETE \
  -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/orgs/YOUR_ORG/packages/container/cliproxyapiplus/versions/VERSION_ID
```

## FAQ

**Q: Do I need to create any secrets?**
A: No! GitHub Container Registry uses the built-in `GITHUB_TOKEN`.

**Q: Are the images public or private?**
A: By default, they inherit the repository visibility. You can change this in package settings.

**Q: How much does it cost?**
A: Free for public repositories. Private repositories have storage limits based on your GitHub plan.

**Q: Can I use both ghcr.io and DockerHub?**
A: Yes! You can push to multiple registries in the same workflow.

**Q: How do I delete old images?**
A: Use the GitHub UI (Packages → Package settings) or the API.

**Q: Can external users pull my images?**
A: Yes, if the package is public. Private packages require authentication.

## Related Documentation

- [GitHub Packages Documentation](https://docs.github.com/en/packages)
- [Working with Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Docker Branch Build Guide](./DOCKER_BRANCH_BUILD.md)
- [GitHub Actions Setup](./GITHUB_ACTIONS_SETUP.md)

---

**Last Updated**: 2024-12-31
**Registry**: ghcr.io (GitHub Container Registry)
**Authentication**: Built-in `GITHUB_TOKEN`
