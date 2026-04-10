# Changelog

## [Unreleased]

### Fixed
- **Gemini CLI "ALL" Selection**: Fixed critical issue where selecting "ALL" during Gemini CLI login would fail silently if any project encountered an error
  - Now skips problematic projects and continues processing others
  - Saves credentials for all successfully activated projects
  - Provides clear warning messages for failed projects
  - Reports summary of successes and failures
  - See [docs/GEMINI_ALL_FIX.md](docs/GEMINI_ALL_FIX.md) for technical details

### Changed
- **Error Handling**: Improved error handling in `internal/cmd/login.go` to distinguish between single-project and multi-project selections
- **Logging**: Enhanced logging with warning messages for skipped projects during batch processing
- **Documentation**: Updated [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) to reflect the fix and new behavior

### Technical Details
- Modified `DoLogin` function in `internal/cmd/login.go`:
  - Added `failedProjects` tracking for project activation phase
  - Added `verifiedProjects` tracking for API verification phase
  - Implemented graceful error handling for multi-project selections
  - Maintained backward compatibility for single-project selections

## Previous Versions

See git history for previous changes.
