# Troubleshooting Guide

## Gemini CLI Authentication Issues

### ✅ FIXED: Credential File Not Created When Selecting "ALL"

> **Status**: This issue has been fixed as of the latest version. The system now properly handles partial failures when selecting "ALL".

#### What Was Fixed

The authentication system now:
- ✅ Skips problematic projects instead of aborting the entire process
- ✅ Saves credentials for successfully activated projects
- ✅ Provides clear warning messages for failed projects
- ✅ Reports which projects succeeded and which failed

#### New Behavior (After Fix)

When you select "ALL" and some projects fail:

```
Activating project project-a
✓ Project project-a activated successfully

Activating project project-b
⚠ Skipping project project-b due to setup error: API not enabled

Activating project project-c
✓ Project project-c activated successfully

⚠ Failed to activate 1 project(s): project-b
✓ Only 2 of 3 projects passed API verification

Authentication saved to ~/.gemini/gemini-user@example.com-all.json
Gemini authentication successful!
```

**Result**: Credentials are saved for `project-a` and `project-c`, even though `project-b` failed.

#### Previous Behavior (Before Fix)

Previously, if ANY project failed:
- ❌ Entire process would abort
- ❌ No credentials saved for any projects
- ❌ Silent failure with no error messages
- ❌ No indication of which project caused the failure

#### Symptoms (Historical - Now Fixed)
- OAuth authentication completes successfully
- User email is confirmed (e.g., `gdtiti@gmail.com`)
- List of projects is displayed (e.g., 72 projects)
- User selects "ALL" to onboard all projects
- Script hangs or exits silently
- No credential file is created
- No error messages are displayed

#### Root Cause (Historical)

**Primary Cause: Silent Failure During Batch Processing**

When selecting "ALL", the Gemini CLI attempted to process all listed projects sequentially. The failure occurred due to:

1. **Unhandled Exceptions in Project Loop** (FIXED)
   - At least one project in the list has an invalid state
   - Missing required APIs (e.g., Generative Language API not enabled)
   - Permission issues on specific projects
   - ~~Without proper error handling, one failed project causes the entire batch to fail silently~~
   - **Now**: Failed projects are skipped with warning messages

2. **Rate Limiting** (Partially Mitigated)
   - Processing many projects in rapid succession may trigger Google Cloud API rate limits
   - No retry logic or exponential backoff implemented
   - ~~Silent timeout without error reporting~~
   - **Now**: Errors are logged with clear messages

3. **Console Output Buffering** (FIXED)
   - ~~Error messages generated but not flushed to console~~
   - **Now**: All warnings and errors are properly logged

4. **State Management Issues** (FIXED)
   - ~~Successfully processed projects not saved incrementally~~
   - ~~All-or-nothing approach loses progress on failure~~
   - **Now**: Successfully activated projects are saved even if others fail

#### Current Best Practices

**Option 1: Use "ALL" Selection (Now Safe)**

You can now safely select "ALL" when prompted:

```bash
# When prompted: Enter project ID [...] or ALL:
ALL
```

The system will:
- Process all projects sequentially
- Skip any problematic projects with warning messages
- Save credentials for all successfully activated projects
- Report a summary of successes and failures

**Option 2: Single Project Selection (Most Reliable)**

For the most reliable experience, choose a single, known-good project:

```bash
# When prompted: Enter project ID [...] or ALL:
# Enter a specific project ID instead of ALL
your-project-id-123
```

**How to choose a project:**
1. Look for projects you actively use
2. Avoid projects with names like `csp-cli-*` (often test/temporary projects)
3. Choose projects where you know the Generative Language API is enabled
4. Prefer projects with simple names without special characters

**Option 3: Enable Required APIs First**

Before running `gemini-cli login`:

1. Visit [Google Cloud Console](https://console.cloud.google.com)
2. For each project you want to use:
   - Enable the "Generative Language API" or "Gemini for Google Cloud API"
   - Verify you have appropriate permissions
   - Check project status is "Active"
3. Then run the login command and select "ALL"

#### Verification Steps

After successful authentication, verify the credential file exists:

**Windows:**
```cmd
dir %USERPROFILE%\.gemini
type %USERPROFILE%\.gemini\gemini-*-all.json
```

**Linux/macOS:**
```bash
ls -la ~/.gemini/
cat ~/.gemini/gemini-*-all.json
```

The credential file should contain a `project_id` field with comma-separated project IDs for all successfully activated projects.

#### Understanding Warning Messages

When using "ALL" selection, you may see these warning messages:

1. **"Skipping project X: project selection required"**
   - The project requires explicit selection and cannot be auto-activated
   - This is normal for certain project types

2. **"Skipping project X due to setup error: [error details]"**
   - The project encountered an error during activation
   - Common causes: API not enabled, permission issues, invalid project state
   - Other projects will continue to be processed

3. **"Failed to activate N project(s): [project list]"**
   - Summary of all projects that failed activation
   - Check each project individually if needed

4. **"Skipping project X: failed to check Cloud AI API"**
   - API verification failed for this project
   - The project may still work, but verification couldn't complete

5. **"Only N of M projects passed API verification"**
   - Some projects failed API verification
   - Only verified projects are saved to credentials

#### When to Report Issues

Report an issue if:
- ✅ ALL projects fail (none succeed)
- ✅ The same project consistently fails with unclear error messages
- ✅ Credentials are not saved despite success messages
- ❌ Some projects fail (this is expected and handled correctly)

#### Technical Details

See [GEMINI_ALL_FIX.md](./GEMINI_ALL_FIX.md) for detailed technical information about the fix.

---

## Other Common Issues

### Issue: "Authentication Failed" Error

**Symptoms:**
- OAuth flow fails immediately
- Browser doesn't open
- "Authentication failed" message

**Solutions:**
1. Check internet connectivity
2. Verify system time is correct (OAuth requires accurate time)
3. Clear browser cookies for Google accounts
4. Try incognito/private browsing mode
5. Check firewall/proxy settings

### Issue: "Project Not Found" Error

**Symptoms:**
- Authentication succeeds
- Specific project cannot be accessed
- "Project not found" or "Permission denied"

**Solutions:**
1. Verify project ID is correct
2. Check you have appropriate IAM permissions
3. Ensure project is active (not deleted or suspended)
4. Confirm you're using the correct Google account

### Issue: Token Expiration

**Symptoms:**
- Previously working authentication stops working
- "Token expired" or "Invalid credentials" errors

**Solutions:**
1. Re-run the login command: `gemini-cli login`
2. Clear cached credentials: `rm -rf ~/.gemini/credentials.json`
3. Complete fresh authentication

---

## Getting Help

For issues not covered here:

1. Check the [Gemini CLI documentation](https://github.com/google/gemini-cli)
2. Search existing issues on GitHub
3. Contact project maintainers with detailed logs
4. Consult the CLIProxyAPIPlus documentation for integration-specific issues
