# Test Plan: Gemini CLI "ALL" Selection Fix

## Test Overview

This document outlines the test plan for verifying the fix to the Gemini CLI "ALL" selection issue.

## Test Environment Setup

### Prerequisites
1. Google Cloud account with multiple projects (at least 3)
2. At least one project with Gemini API disabled
3. At least one project with Gemini API enabled
4. CLIProxyAPIPlus backend built and ready
5. Clean credential state (remove existing `~/.gemini/` directory)

### Environment Variables
```bash
# Backend
BACKEND_PORT=8080
AUTH_DIR=~/.gemini

# Test Projects (example)
PROJECT_ENABLED_1=my-active-project-1
PROJECT_ENABLED_2=my-active-project-2
PROJECT_DISABLED=my-test-project-disabled
```

## Test Cases

### TC-001: ALL Selection with All Projects Successful

**Objective**: Verify that ALL selection works when all projects are valid

**Preconditions**:
- All projects have Gemini API enabled
- No credential file exists

**Steps**:
1. Run `gemini-cli login`
2. Complete OAuth flow
3. When prompted, enter "ALL"
4. Wait for completion

**Expected Results**:
- ✅ All projects are activated successfully
- ✅ No warning messages about skipped projects
- ✅ Credential file created: `~/.gemini/gemini-<email>-all.json`
- ✅ `project_id` field contains all project IDs (comma-separated)
- ✅ Success message: "Gemini authentication successful!"

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### TC-002: ALL Selection with Partial Failures

**Objective**: Verify that ALL selection saves credentials for successful projects when some fail

**Preconditions**:
- At least 2 projects with Gemini API enabled
- At least 1 project with Gemini API disabled
- No credential file exists

**Steps**:
1. Run `gemini-cli login`
2. Complete OAuth flow
3. When prompted, enter "ALL"
4. Observe console output
5. Check credential file

**Expected Results**:
- ✅ Successful projects are activated
- ✅ Warning messages appear for failed projects:
  - "Skipping project X due to setup error: ..."
  - "Failed to activate N project(s): ..."
- ✅ Credential file created with only successful projects
- ✅ `project_id` field contains only successful project IDs
- ✅ Success message: "Gemini authentication successful!"

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### TC-003: ALL Selection with Complete Failure

**Objective**: Verify that ALL selection aborts when all projects fail

**Preconditions**:
- All projects have Gemini API disabled or have errors
- No credential file exists

**Steps**:
1. Run `gemini-cli login`
2. Complete OAuth flow
3. When prompted, enter "ALL"
4. Observe console output

**Expected Results**:
- ✅ Warning messages for each failed project
- ✅ Error message: "No projects were successfully activated; aborting login."
- ✅ No credential file created
- ✅ Process exits with error

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### TC-004: Single Project Selection with Success

**Objective**: Verify single project selection still works (regression test)

**Preconditions**:
- At least 1 project with Gemini API enabled
- No credential file exists

**Steps**:
1. Run `gemini-cli login`
2. Complete OAuth flow
3. When prompted, enter a specific project ID
4. Wait for completion

**Expected Results**:
- ✅ Project is activated successfully
- ✅ No warning messages
- ✅ Credential file created: `~/.gemini/gemini-<email>-<project-id>.json`
- ✅ `project_id` field contains the selected project ID
- ✅ Success message: "Gemini authentication successful!"

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### TC-005: Single Project Selection with Failure

**Objective**: Verify single project selection fails completely on error (regression test)

**Preconditions**:
- At least 1 project with Gemini API disabled
- No credential file exists

**Steps**:
1. Run `gemini-cli login`
2. Complete OAuth flow
3. When prompted, enter a project ID with disabled API
4. Observe console output

**Expected Results**:
- ✅ Error message: "Failed to complete user setup: ..."
- ✅ No credential file created
- ✅ Process exits with error
- ✅ No partial credentials saved

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### TC-006: API Verification Phase Failures

**Objective**: Verify that API verification failures are handled correctly

**Preconditions**:
- Multiple projects that pass activation but fail API verification
- No credential file exists

**Steps**:
1. Run `gemini-cli login`
2. Complete OAuth flow
3. When prompted, enter "ALL"
4. Observe API verification phase

**Expected Results**:
- ✅ Projects pass activation phase
- ✅ Warning messages for API verification failures:
  - "Skipping project X: failed to check Cloud AI API: ..."
  - "Only N of M projects passed API verification"
- ✅ Credential file contains only verified projects
- ✅ Success message appears

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### TC-007: Credential File Format Validation

**Objective**: Verify credential file format is correct

**Preconditions**:
- Successful ALL selection with multiple projects

**Steps**:
1. Complete TC-002 (ALL with partial failures)
2. Read credential file: `cat ~/.gemini/gemini-*-all.json`
3. Validate JSON structure

**Expected Results**:
- ✅ Valid JSON format
- ✅ Contains `token` object with OAuth2 credentials
- ✅ Contains `project_id` field with comma-separated IDs
- ✅ Contains `email` field with user email
- ✅ Contains `type: "gemini"`
- ✅ Contains `checked: true`
- ✅ Contains `auto: false`

**Example**:
```json
{
  "token": { ... },
  "project_id": "project-a,project-c",
  "email": "user@example.com",
  "auto": false,
  "checked": true,
  "type": "gemini"
}
```

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### TC-008: Warning Message Clarity

**Objective**: Verify warning messages are clear and actionable

**Preconditions**:
- ALL selection with at least one failing project

**Steps**:
1. Complete TC-002 (ALL with partial failures)
2. Review all console output
3. Verify warning messages

**Expected Results**:
- ✅ Each failed project has a clear warning message
- ✅ Warning includes project ID
- ✅ Warning includes reason for failure
- ✅ Summary message lists all failed projects
- ✅ Summary message counts failed projects
- ✅ Messages use appropriate log levels (Warn, Error, Info)

**Example Output**:
```
⚠ Skipping project test-project: API not enabled
⚠ Failed to activate 1 project(s): test-project
```

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### TC-009: Backend API Integration

**Objective**: Verify backend can read and use the saved credentials

**Preconditions**:
- Successful ALL selection with credentials saved
- Backend server running

**Steps**:
1. Complete TC-002 (ALL with partial failures)
2. Start backend server
3. Make API request using saved credentials
4. Verify backend can authenticate

**Expected Results**:
- ✅ Backend loads credential file successfully
- ✅ Backend can parse comma-separated project IDs
- ✅ Backend can make authenticated requests
- ✅ No errors in backend logs

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### TC-010: Concurrent Project Processing

**Objective**: Verify behavior with many projects (stress test)

**Preconditions**:
- Google Cloud account with 10+ projects
- Mix of enabled and disabled APIs

**Steps**:
1. Run `gemini-cli login`
2. Complete OAuth flow
3. When prompted, enter "ALL"
4. Monitor console output and timing

**Expected Results**:
- ✅ All projects are processed sequentially
- ✅ Failed projects are skipped with warnings
- ✅ Successful projects are saved
- ✅ Process completes within reasonable time
- ✅ No crashes or hangs
- ✅ Memory usage remains stable

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

## Performance Tests

### PT-001: Login Time with ALL Selection

**Objective**: Measure login time with multiple projects

**Test Data**:
- 3 projects: ~X seconds
- 5 projects: ~Y seconds
- 10 projects: ~Z seconds

**Acceptance Criteria**:
- Average time per project: < 10 seconds
- Total time for 10 projects: < 2 minutes

**Results**: _[To be filled during testing]_

---

### PT-002: Memory Usage

**Objective**: Verify no memory leaks during batch processing

**Steps**:
1. Monitor memory before login
2. Process ALL selection with 10+ projects
3. Monitor memory after completion

**Acceptance Criteria**:
- Memory increase: < 50MB
- No memory leaks after completion

**Results**: _[To be filled during testing]_

---

## Security Tests

### ST-001: Credential File Permissions

**Objective**: Verify credential file has correct permissions

**Steps**:
1. Complete successful login
2. Check file permissions: `ls -la ~/.gemini/`

**Expected Results**:
- ✅ File permissions: 600 (rw-------)
- ✅ Directory permissions: 700 (rwx------)
- ✅ Owner: current user

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### ST-002: Sensitive Data in Logs

**Objective**: Verify no sensitive data is logged

**Steps**:
1. Enable debug logging
2. Complete login with failures
3. Review all log output

**Expected Results**:
- ✅ No OAuth tokens in logs
- ✅ No refresh tokens in logs
- ✅ No access tokens in logs
- ✅ Project IDs are logged (acceptable)
- ✅ Email addresses are logged (acceptable)

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

## Regression Tests

### RT-001: Existing Credential Files

**Objective**: Verify existing credential files are not affected

**Preconditions**:
- Existing valid credential file

**Steps**:
1. Backup existing credential file
2. Run new login with ALL selection
3. Compare old and new files

**Expected Results**:
- ✅ Old credential file is replaced (expected behavior)
- ✅ New credential file has correct format
- ✅ No corruption of credential data

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### RT-002: Backend Compatibility

**Objective**: Verify backend works with both old and new credential formats

**Steps**:
1. Test backend with old single-project credential
2. Test backend with new multi-project credential
3. Verify both work correctly

**Expected Results**:
- ✅ Backend reads old format correctly
- ✅ Backend reads new format correctly
- ✅ No breaking changes

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

## Edge Cases

### EC-001: Empty Project List

**Objective**: Verify behavior when no projects are available

**Preconditions**:
- Google Cloud account with no projects

**Steps**:
1. Run `gemini-cli login`
2. Complete OAuth flow
3. Observe behavior

**Expected Results**:
- ✅ Message: "No Google Cloud projects are available for selection."
- ✅ Process exits gracefully
- ✅ No credential file created

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### EC-002: Network Interruption

**Objective**: Verify behavior when network is interrupted

**Steps**:
1. Start login with ALL selection
2. Disconnect network during processing
3. Observe behavior

**Expected Results**:
- ✅ Error message about network failure
- ✅ Partial credentials may be saved (for projects processed before interruption)
- ✅ Clear error message about which projects failed

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

### EC-003: Special Characters in Project IDs

**Objective**: Verify handling of project IDs with special characters

**Preconditions**:
- Projects with hyphens, underscores, numbers

**Steps**:
1. Run login with ALL selection
2. Verify all project types are handled

**Expected Results**:
- ✅ All valid project ID formats are handled
- ✅ Comma-separated list is correctly formatted
- ✅ No parsing errors

**Actual Results**: _[To be filled during testing]_

**Status**: _[Pass/Fail]_

---

## Test Execution Summary

### Test Statistics
- Total Test Cases: 18
- Passed: _[To be filled]_
- Failed: _[To be filled]_
- Blocked: _[To be filled]_
- Not Executed: _[To be filled]_

### Critical Issues Found
_[To be filled during testing]_

### Non-Critical Issues Found
_[To be filled during testing]_

### Test Environment Details
- OS: _[To be filled]_
- Go Version: _[To be filled]_
- Backend Version: _[To be filled]_
- Test Date: _[To be filled]_
- Tester: _[To be filled]_

### Sign-off

**Tested By**: ________________
**Date**: ________________
**Approved By**: ________________
**Date**: ________________

---

## Automated Test Script (Future)

```bash
#!/bin/bash
# test-gemini-all-fix.sh
# Automated test script for Gemini CLI ALL selection fix

set -e

echo "=== Gemini CLI ALL Selection Fix - Automated Tests ==="

# Setup
export TEST_PROJECT_1="project-enabled-1"
export TEST_PROJECT_2="project-enabled-2"
export TEST_PROJECT_DISABLED="project-disabled"

# Clean state
rm -rf ~/.gemini/

# TC-001: All projects successful
echo "Running TC-001: ALL Selection with All Projects Successful"
# [Test implementation]

# TC-002: Partial failures
echo "Running TC-002: ALL Selection with Partial Failures"
# [Test implementation]

# TC-003: Complete failure
echo "Running TC-003: ALL Selection with Complete Failure"
# [Test implementation]

# TC-004: Single project success
echo "Running TC-004: Single Project Selection with Success"
# [Test implementation]

# TC-005: Single project failure
echo "Running TC-005: Single Project Selection with Failure"
# [Test implementation]

echo "=== All tests completed ==="
```

---

## Notes

1. **Manual Testing Required**: Due to OAuth flow, most tests require manual execution
2. **Test Data**: Ensure test projects are properly configured before testing
3. **Cleanup**: Remove credential files between tests for clean state
4. **Documentation**: Update this document with actual results during testing
5. **Automation**: Consider implementing automated tests for backend credential parsing
