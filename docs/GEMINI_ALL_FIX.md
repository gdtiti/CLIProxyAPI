# Gemini CLI "ALL" Selection Fix

## Problem Description

When using Gemini CLI login with "ALL" project selection, if any single project encountered an error during activation or API verification:
- The entire login process would abort immediately
- No credentials would be saved for any projects (including successful ones)
- No clear error message indicating which project failed
- Backend would silently fail without proper error reporting

## Root Cause

The login flow in `internal/cmd/login.go` had two critical issues:

1. **Project Activation Loop (lines 118-136)**: Used `return` on any error, terminating the entire process
2. **API Verification Loop (lines 167-199)**: Same issue - any failed project would abort the entire login

## Solution

Modified the error handling to distinguish between single-project and multi-project selections:

### 1. Project Activation Phase

**Before:**
```go
for _, candidateID := range projectSelections {
    if errSetup := performGeminiCLISetup(...); errSetup != nil {
        log.Errorf("Failed to complete user setup: %v", errSetup)
        return  // ❌ Aborts entire process
    }
    activatedProjects = append(activatedProjects, finalID)
}
```

**After:**
```go
activatedProjects := make([]string, 0, len(projectSelections))
failedProjects := make([]string, 0)

for _, candidateID := range projectSelections {
    if errSetup := performGeminiCLISetup(...); errSetup != nil {
        // For multi-project selection, skip and continue
        if len(projectSelections) > 1 {
            log.Warnf("Skipping project %s due to setup error: %v", candidateID, errSetup)
            failedProjects = append(failedProjects, candidateID)
            continue  // ✅ Skip failed project, continue with others
        }
        // For single project, still fatal
        log.Errorf("Failed to complete user setup: %v", errSetup)
        return
    }
    activatedProjects = append(activatedProjects, finalID)
}

// Report failures and check if any succeeded
if len(failedProjects) > 0 {
    log.Warnf("Failed to activate %d project(s): %s", len(failedProjects), strings.Join(failedProjects, ", "))
}
if len(activatedProjects) == 0 {
    log.Error("No projects were successfully activated; aborting login.")
    return
}
```

### 2. API Verification Phase

**Before:**
```go
for _, pid := range activatedProjects {
    isChecked, errCheck := checkCloudAPIIsEnabled(...)
    if errCheck != nil {
        log.Errorf("Failed to check if Cloud AI API is enabled for %s: %v", pid, errCheck)
        return  // ❌ Aborts entire process
    }
}
```

**After:**
```go
verifiedProjects := make([]string, 0, len(activatedProjects))

for _, pid := range activatedProjects {
    isChecked, errCheck := checkCloudAPIIsEnabled(...)
    if errCheck != nil {
        // For multi-project, skip and continue
        if len(activatedProjects) > 1 {
            log.Warnf("Skipping project %s: failed to check Cloud AI API: %v", pid, errCheck)
            continue  // ✅ Skip failed project
        }
        log.Errorf("Failed to check if Cloud AI API is enabled for %s: %v", pid, errCheck)
        return
    }
    verifiedProjects = append(verifiedProjects, pid)
}

// Update storage with only verified projects
if len(verifiedProjects) < len(activatedProjects) {
    log.Warnf("Only %d of %d projects passed API verification", len(verifiedProjects), len(activatedProjects))
    activatedProjects = verifiedProjects
    storage.ProjectID = strings.Join(activatedProjects, ",")
}
```

## Behavior After Fix

### Scenario 1: ALL Selection with Mixed Results

```
User selects: ALL (4 projects)

Processing:
✓ project-a: Activation successful
✗ project-b: Activation failed (API not enabled)
✓ project-c: Activation successful
✗ project-d: API verification failed

Result:
- Credentials saved for: project-a, project-c
- Warning logged: "Failed to activate 1 project(s): project-b"
- Warning logged: "Only 2 of 3 projects passed API verification"
- Success message: "Gemini authentication successful!"
- Saved file: gemini-user@example.com-all.json (contains project-a,project-c)
```

### Scenario 2: Single Project Selection (Unchanged)

```
User selects: project-a

Processing:
✗ project-a: Activation failed

Result:
- No credentials saved
- Error logged: "Failed to complete user setup: <error details>"
- Process aborts (same as before)
```

### Scenario 3: ALL Selection with Complete Failure

```
User selects: ALL (3 projects)

Processing:
✗ project-a: Activation failed
✗ project-b: Activation failed
✗ project-c: Activation failed

Result:
- No credentials saved
- Warning logged: "Failed to activate 3 project(s): project-a, project-b, project-c"
- Error logged: "No projects were successfully activated; aborting login."
- Process aborts
```

## Testing

### Manual Testing Steps

1. **Test ALL with partial success:**
   ```bash
   # Ensure you have multiple GCP projects, some with API disabled
   gemini-cli login
   # Select "ALL" when prompted
   # Verify: Credentials saved for successful projects only
   # Verify: Warning messages for failed projects
   ```

2. **Test single project (regression):**
   ```bash
   gemini-cli login
   # Select a single project
   # Verify: Behavior unchanged (fails completely on error)
   ```

3. **Verify saved credentials:**
   ```bash
   # Check the saved file
   cat ~/.gemini/gemini-<email>-all.json
   # Verify: Only successful projects listed in project_id field
   ```

## Files Modified

- `internal/cmd/login.go`: Lines 118-199
  - Added `failedProjects` tracking
  - Added `verifiedProjects` tracking
  - Modified error handling to skip failed projects in multi-project mode
  - Added warning messages for skipped projects
  - Added validation to ensure at least one project succeeds

## Related Issues

- Fixes: Silent failure when selecting ALL with problematic projects
- Fixes: No credentials saved despite some projects succeeding
- Fixes: Lack of error reporting for which projects failed

## Backward Compatibility

✅ **Fully backward compatible**
- Single project selection behavior unchanged
- Existing credential files remain valid
- No API changes
- No configuration changes required

## Additional Notes

- The fix maintains the existing credential file naming convention
- Failed projects are logged with `log.Warnf()` for visibility
- The final saved credential contains only successfully activated and verified projects
- Users can still see which projects failed in the logs
