# Quick Start Guide: Testing the Gemini CLI "ALL" Fix

This guide helps you quickly test the Gemini CLI "ALL" selection fix in the `CLIProxyAPIPlus-gdtiti` branch.

## Prerequisites

- Google Cloud account with multiple projects (at least 2-3)
- At least one project with Gemini API disabled (for testing the fix)
- Docker installed (for easiest testing) OR Go 1.24+ (for building from source)

## Option 1: Test with Docker (Recommended)

### Step 1: Pull the Development Image

```bash
docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti
```

### Step 2: Run the Container

```bash
docker run -it --rm \
  --name cliproxyapi-test \
  -p 8317:8317 \
  -v ~/.gemini:/root/.gemini \
  eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti sh
```

### Step 3: Login with Gemini CLI

Inside the container:

```bash
# Start the login process
gemini-cli login

# Follow the OAuth flow in your browser
# When prompted for project selection, type: ALL
```

### Step 4: Observe the Fix in Action

You should see output like this:

```
Activating project my-project-1
✓ Project my-project-1 activated successfully

Activating project test-project-disabled
⚠ Skipping project test-project-disabled due to setup error: API not enabled

Activating project my-project-2
✓ Project my-project-2 activated successfully

⚠ Failed to activate 1 project(s): test-project-disabled
✓ Authentication saved to /root/.gemini/gemini-user@example.com-all.json
Gemini authentication successful!
```

### Step 5: Verify Credentials

```bash
# Check the credential file
cat /root/.gemini/gemini-*.json | grep project_id

# Should show only successful projects:
# "project_id": "my-project-1,my-project-2"
```

### Step 6: Test the Backend

Exit the container and start the backend:

```bash
# Exit the interactive shell
exit

# Run the backend server
docker run -d \
  --name cliproxyapi-backend \
  -p 8317:8317 \
  -v ~/.gemini:/root/.gemini \
  eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti

# Check logs
docker logs -f cliproxyapi-backend
```

## Option 2: Test from Source

### Step 1: Clone and Checkout

```bash
# Clone the repository
git clone https://github.com/your-org/CLIProxyAPIPlus.git
cd CLIProxyAPIPlus

# Checkout the fix branch
git checkout CLIProxyAPIPlus-gdtiti
```

### Step 2: Build the Backend

```bash
# Install dependencies
go mod download

# Build
go build -o bin/cliproxyapi cmd/server/main.go
```

### Step 3: Login with Gemini CLI

```bash
# Make sure gemini-cli is installed
# If not: npm install -g @google/generative-ai-cli

# Login
gemini-cli login

# Select "ALL" when prompted
```

### Step 4: Observe the Fix

Watch for the same output as in the Docker example above.

### Step 5: Start the Backend

```bash
# Run the backend
./bin/cliproxyapi

# Or use the dev script
./dev.sh dev
```

## Option 3: Quick Test with GitHub Actions

If you have push access to the repository:

### Step 1: Trigger the Build

```bash
# Push to the branch (if you have changes)
git push origin CLIProxyAPIPlus-gdtiti

# Or manually trigger via GitHub UI:
# 1. Go to Actions tab
# 2. Select "Docker Branch Build"
# 3. Click "Run workflow"
# 4. Select branch: CLIProxyAPIPlus-gdtiti
# 5. Click "Run workflow"
```

### Step 2: Wait for Build

The build takes about 5-10 minutes. Monitor progress in the Actions tab.

### Step 3: Pull and Test

Once the build completes:

```bash
docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti
# Then follow Option 1 steps above
```

## What to Test

### Test Case 1: ALL with Mixed Results ✅

**Setup**: Have projects with both enabled and disabled APIs

**Expected**:
- Successful projects are saved
- Failed projects show warnings
- Credential file contains only successful projects

### Test Case 2: ALL with All Success ✅

**Setup**: All projects have APIs enabled

**Expected**:
- All projects are activated
- No warning messages
- All projects in credential file

### Test Case 3: ALL with All Failures ❌

**Setup**: All projects have APIs disabled

**Expected**:
- All projects show warnings
- Error: "No projects were successfully activated"
- No credential file created

### Test Case 4: Single Project (Regression) ✅

**Setup**: Select a single project instead of ALL

**Expected**:
- Behavior unchanged from before
- If project fails, entire process fails
- No partial credentials

## Verification Checklist

- [ ] OAuth flow completes successfully
- [ ] Project list is displayed
- [ ] "ALL" option is accepted
- [ ] Failed projects show warning messages with project IDs
- [ ] Successful projects show success messages
- [ ] Summary message shows count of failed projects
- [ ] Credential file is created
- [ ] Credential file contains only successful project IDs
- [ ] Backend can read and use the credentials
- [ ] No crashes or hangs during the process

## Common Issues and Solutions

### Issue: No Projects Listed

**Solution**: Ensure your Google account has access to GCP projects

```bash
# Check your Google Cloud projects
gcloud projects list
```

### Issue: All Projects Fail

**Solution**: Enable the required APIs

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. For each project, enable:
   - Generative Language API
   - Gemini for Google Cloud API

### Issue: Docker Image Not Found

**Solution**: Check if the build completed

```bash
# Check DockerHub
curl -s https://hub.docker.com/v2/repositories/eceasy/cli-proxy-api-plus/tags | grep cliproxyapiplus-gdtiti

# Or trigger a new build via GitHub Actions
```

### Issue: Credentials Not Saved

**Solution**: Check volume mounting

```bash
# Ensure the volume is correctly mounted
docker run -it --rm \
  -v ~/.gemini:/root/.gemini \
  eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti \
  ls -la /root/.gemini
```

## Comparing with Old Behavior

### Before the Fix

```bash
# Old behavior (for reference only - don't test this)
# If ANY project failed:
# - Process aborted immediately
# - No credentials saved
# - No error messages
# - Silent failure
```

### After the Fix

```bash
# New behavior (current branch)
# If some projects fail:
# - Failed projects are skipped with warnings
# - Successful projects are saved
# - Clear error messages
# - Summary of results
```

## Performance Testing

### Test with Many Projects

If you have 10+ projects:

```bash
# Time the login process
time gemini-cli login
# Select "ALL"

# Expected: ~5-10 seconds per project
# Total time: ~1-2 minutes for 10 projects
```

### Monitor Resource Usage

```bash
# In another terminal
docker stats cliproxyapi-test

# Expected:
# - CPU: < 50%
# - Memory: < 100MB
```

## Next Steps

After successful testing:

1. **Report Results**: Share your test results in the GitHub issue
2. **Test Integration**: Try using the credentials with your application
3. **Test Edge Cases**: Try with different project configurations
4. **Provide Feedback**: Suggest improvements or report issues

## Getting Help

If you encounter issues:

1. **Check Logs**:
   ```bash
   docker logs cliproxyapi-test
   ```

2. **Check Documentation**:
   - [GEMINI_ALL_FIX.md](./GEMINI_ALL_FIX.md) - Technical details
   - [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) - Common issues
   - [TEST_PLAN.md](./TEST_PLAN.md) - Detailed test cases

3. **Create an Issue**:
   - Include your test results
   - Attach logs
   - Describe your environment

## Automated Testing Script

Save this as `test-gemini-fix.sh`:

```bash
#!/bin/bash
set -e

echo "=== Testing Gemini CLI ALL Fix ==="
echo ""

# Pull latest image
echo "1. Pulling latest development image..."
docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti

# Start container
echo "2. Starting test container..."
docker run -d \
  --name cliproxyapi-test \
  -p 8317:8317 \
  -v ~/.gemini:/root/.gemini \
  eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti

# Wait for startup
echo "3. Waiting for container to start..."
sleep 5

# Check if running
if docker ps | grep -q cliproxyapi-test; then
  echo "✓ Container is running"
else
  echo "✗ Container failed to start"
  exit 1
fi

# Check logs
echo "4. Checking logs..."
docker logs cliproxyapi-test

# Prompt for manual testing
echo ""
echo "5. Manual testing required:"
echo "   Run: docker exec -it cliproxyapi-test sh"
echo "   Then: gemini-cli login"
echo "   Select: ALL"
echo ""
echo "Press Enter when done testing..."
read

# Check credentials
echo "6. Checking credentials..."
if docker exec cliproxyapi-test ls /root/.gemini/*.json > /dev/null 2>&1; then
  echo "✓ Credential file found"
  docker exec cliproxyapi-test cat /root/.gemini/*.json | grep project_id
else
  echo "✗ No credential file found"
fi

# Cleanup
echo ""
echo "7. Cleanup (y/n)?"
read -r response
if [[ "$response" =~ ^[Yy]$ ]]; then
  docker stop cliproxyapi-test
  docker rm cliproxyapi-test
  echo "✓ Cleanup complete"
fi

echo ""
echo "=== Testing Complete ==="
```

Make it executable and run:

```bash
chmod +x test-gemini-fix.sh
./test-gemini-fix.sh
```

## Summary

This fix ensures that:
- ✅ Partial failures don't block the entire login process
- ✅ Successful projects are always saved
- ✅ Clear feedback about what succeeded and what failed
- ✅ Better user experience with informative messages

**Ready to test?** Start with Option 1 (Docker) for the quickest experience!
