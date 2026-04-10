# GitHub Actions Setup for Docker Branch Builds

This guide explains how to configure GitHub Actions to automatically build Docker images from your development branches.

## Prerequisites

- GitHub repository with admin access
- DockerHub account
- Repository secrets configured

## Step 1: Configure Repository Secrets

### Required Secrets

Go to your GitHub repository → Settings → Secrets and variables → Actions → New repository secret

Add the following secrets:

| Secret Name | Description | How to Get |
|-------------|-------------|------------|
| `DOCKERHUB_USERNAME` | Your DockerHub username | Your DockerHub login name |
| `DOCKERHUB_TOKEN` | DockerHub access token | Create at https://hub.docker.com/settings/security |

### Creating a DockerHub Access Token

1. Go to https://hub.docker.com/settings/security
2. Click **New Access Token**
3. Name: `GitHub Actions - CLIProxyAPIPlus`
4. Permissions: **Read, Write, Delete**
5. Click **Generate**
6. Copy the token (you won't see it again!)
7. Add it to GitHub secrets as `DOCKERHUB_TOKEN`

## Step 2: Verify Workflow File

The workflow file should already exist at `.github/workflows/docker-branch.yml`

### Workflow Configuration

```yaml
name: Docker Branch Build

on:
  push:
    branches:
      - 'CLIProxyAPIPlus-gdtiti'
      - 'feature/**'
      - 'fix/**'
      - 'dev/**'
  workflow_dispatch:
    inputs:
      branch:
        description: 'Branch to build (leave empty for current branch)'
        required: false
        type: string
```

### Supported Triggers

1. **Automatic on Push**: Builds automatically when you push to supported branches
2. **Manual Dispatch**: Trigger builds manually from GitHub UI

## Step 3: Test the Workflow

### Method 1: Push to Branch

```bash
# Make a small change
echo "# Test" >> README.md
git add README.md
git commit -m "Test Docker build"
git push origin CLIProxyAPIPlus-gdtiti
```

### Method 2: Manual Trigger

1. Go to your repository on GitHub
2. Click **Actions** tab
3. Select **Docker Branch Build** workflow
4. Click **Run workflow** button
5. (Optional) Enter a branch name or leave empty
6. Click **Run workflow**

## Step 4: Monitor the Build

### View Build Progress

1. Go to **Actions** tab
2. Click on the running workflow
3. Click on the **docker-branch** job
4. Expand each step to see logs

### Expected Steps

1. ✅ Checkout - Clone the repository
2. ✅ Set up QEMU - Multi-platform support
3. ✅ Set up Docker Buildx - Advanced build features
4. ✅ Login to DockerHub - Authenticate
5. ✅ Generate Build Metadata - Version info
6. ✅ Docker meta - Generate tags
7. ✅ Build and push - Build for amd64 and arm64
8. ✅ Image digest - Show build result

### Build Time

- First build: ~10-15 minutes (no cache)
- Subsequent builds: ~5-8 minutes (with cache)

## Step 5: Verify the Image

### Check DockerHub

1. Go to https://hub.docker.com/r/eceasy/cli-proxy-api-plus/tags
2. Look for your branch tag (e.g., `cliproxyapiplus-gdtiti`)
3. Verify the image was pushed recently

### Pull and Test

```bash
# Pull the image
docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti

# Verify it works
docker run --rm eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti ./CLIProxyAPIPlus --version
```

## Troubleshooting

### Build Fails: Authentication Error

**Error**: `Error: Cannot perform an interactive login from a non TTY device`

**Solution**: Check that `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets are correctly set

```bash
# Verify secrets exist (in GitHub UI)
Settings → Secrets and variables → Actions
# Should see: DOCKERHUB_USERNAME, DOCKERHUB_TOKEN
```

### Build Fails: Permission Denied

**Error**: `denied: requested access to the resource is denied`

**Solution**:
1. Verify DockerHub token has **Write** permissions
2. Verify the repository name in workflow matches your DockerHub repo
3. Check if the DockerHub repository exists

### Build Fails: Dockerfile Not Found

**Error**: `failed to solve: failed to read dockerfile`

**Solution**: Ensure `Dockerfile` exists in the repository root

```bash
# Check if Dockerfile exists
ls -la Dockerfile

# If missing, create it or check the branch
git checkout CLIProxyAPIPlus-gdtiti
```

### Build Timeout

**Error**: Build exceeds 6 hours (GitHub Actions limit)

**Solution**:
1. Check if the build is stuck on a step
2. Cancel and retry
3. Check for network issues

### Image Not Found After Build

**Problem**: Build succeeds but image not on DockerHub

**Solution**:
1. Check the workflow logs for the "Build and push" step
2. Verify `push: true` is set in the workflow
3. Check DockerHub repository permissions

### Wrong Image Tag

**Problem**: Image has unexpected tag name

**Solution**: Check the branch name sanitization in workflow

```yaml
# Branch names are sanitized:
# CLIProxyAPIPlus-gdtiti → cliproxyapiplus-gdtiti
# feature/my-feature → feature-my-feature
```

## Advanced Configuration

### Add More Branches

Edit `.github/workflows/docker-branch.yml`:

```yaml
on:
  push:
    branches:
      - 'CLIProxyAPIPlus-gdtiti'
      - 'feature/**'
      - 'fix/**'
      - 'dev/**'
      - 'your-branch-name'  # Add your branch here
```

### Change Docker Repository

Edit the workflow file:

```yaml
env:
  DOCKERHUB_REPO: your-username/your-repo-name
```

### Add Build Notifications

Add a notification step to the workflow:

```yaml
- name: Notify on Success
  if: success()
  run: |
    curl -X POST https://your-webhook-url \
      -H 'Content-Type: application/json' \
      -d '{"text":"Docker build succeeded for ${{ github.ref_name }}"}'
```

### Build for More Platforms

Edit the workflow file:

```yaml
platforms: |
  linux/amd64
  linux/arm64
  linux/arm/v7  # Add more platforms
```

## Workflow Customization

### Change Build Frequency

**Option 1: Build on Every Commit**
```yaml
on:
  push:
    branches:
      - 'CLIProxyAPIPlus-gdtiti'
```

**Option 2: Build on Tag Only**
```yaml
on:
  push:
    tags:
      - 'dev-*'
```

**Option 3: Scheduled Builds**
```yaml
on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM UTC
```

### Add Build Matrix

Build multiple variants:

```yaml
strategy:
  matrix:
    variant: [alpine, debian]

steps:
  - name: Build and push
    uses: docker/build-push-action@v6
    with:
      file: Dockerfile.${{ matrix.variant }}
      tags: ${{ env.DOCKERHUB_REPO }}:${{ env.SAFE_BRANCH }}-${{ matrix.variant }}
```

## Security Best Practices

### 1. Use Access Tokens, Not Passwords

✅ **Good**: Use DockerHub access tokens
❌ **Bad**: Use your DockerHub password

### 2. Limit Token Permissions

Create tokens with minimal required permissions:
- Read, Write (for pushing images)
- Avoid "Admin" permissions

### 3. Rotate Tokens Regularly

1. Create a new token
2. Update GitHub secret
3. Delete old token

### 4. Use Dependabot

Enable Dependabot to keep actions up to date:

```yaml
# .github/dependabot.yml
version: 2
updates:
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
```

### 5. Scan Images for Vulnerabilities

Add a security scan step:

```yaml
- name: Run Trivy vulnerability scanner
  uses: aquasecurity/trivy-action@master
  with:
    image-ref: ${{ env.DOCKERHUB_REPO }}:${{ env.SAFE_BRANCH }}
    format: 'sarif'
    output: 'trivy-results.sarif'
```

## Monitoring and Maintenance

### Check Build Status

**GitHub Badge**: Add to README.md

```markdown
![Docker Branch Build](https://github.com/your-org/CLIProxyAPIPlus/actions/workflows/docker-branch.yml/badge.svg?branch=CLIProxyAPIPlus-gdtiti)
```

### Monitor DockerHub Usage

1. Go to https://hub.docker.com/settings/billing
2. Check pull/push limits
3. Monitor storage usage

### Clean Up Old Images

**Manual Cleanup**:
```bash
# List all tags
curl -s https://hub.docker.com/v2/repositories/eceasy/cli-proxy-api-plus/tags | jq -r '.results[].name'

# Delete old tags (via DockerHub UI or API)
```

**Automated Cleanup**: Add a cleanup workflow

```yaml
name: Cleanup Old Images

on:
  schedule:
    - cron: '0 0 * * 0'  # Weekly

jobs:
  cleanup:
    runs-on: ubuntu-latest
    steps:
      - name: Delete old branch images
        run: |
          # Keep only last 5 builds per branch
          # Implementation depends on your needs
```

## Cost Considerations

### GitHub Actions

- **Free tier**: 2,000 minutes/month for private repos
- **Public repos**: Unlimited
- **Build time**: ~10 minutes per build
- **Estimate**: ~200 builds/month on free tier

### DockerHub

- **Free tier**: Unlimited public repositories
- **Pull limits**: 200 pulls per 6 hours (anonymous), unlimited (authenticated)
- **Storage**: Unlimited for public repos

## Next Steps

1. ✅ Configure secrets
2. ✅ Test the workflow
3. ✅ Verify image on DockerHub
4. ✅ Update documentation
5. ✅ Set up notifications (optional)
6. ✅ Enable Dependabot (optional)
7. ✅ Add security scanning (optional)

## Related Documentation

- [Docker Branch Build Guide](./DOCKER_BRANCH_BUILD.md) - Using branch images
- [Quick Start Testing](./QUICK_START_TESTING.md) - Testing the fix
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Docker Build Push Action](https://github.com/docker/build-push-action)

## Support

For issues with GitHub Actions setup:
1. Check workflow logs in Actions tab
2. Review GitHub Actions documentation
3. Check DockerHub status page
4. Create an issue with workflow logs

---

**Last Updated**: 2024-12-31
**Workflow File**: `.github/workflows/docker-branch.yml`
