# Auth Files Batch Excluded Models API

## Purpose

This API is used by the management UI to batch update `excluded_models` for multiple auth JSON credential files after users search/filter/select in the credential list.

Typical UI flow:

1. Frontend loads credential list from `/v0/management/auth-files`.
2. User filters/searches/selects multiple credential files.
3. Frontend calls this batch API once with selected file names.
4. Backend updates each selected JSON file and refreshes in-memory auth state.

## Endpoint

- Method: `PATCH`
- Path: `/v0/management/auth-files/excluded-models/batch`
- Auth: same management key auth as other `/v0/management/*` endpoints

## Request Body

```json
{
  "names": ["claude-user-a.json", "gemini-user-b.json"],
  "operation": "add",
  "models": ["gpt-4o*", "claude-3.7-sonnet"],
  "dry_run": false,
  "stop_on_error": false
}
```

### Fields

- `names`:
  - Type: `string[]`
  - Required: yes
  - Description: selected auth file names (must be `.json`, file name only, no path separators)
  - Limit: max 500 items
- `operation`:
  - Type: `string`
  - Required: no (default `set`)
  - Allowed:
    - `set`: replace with `models`
    - `add`: merge existing list + `models`
    - `remove`: remove `models` from existing list
    - `clear`: remove `excluded_models` from file
- `models`:
  - Type: `string[]`
  - Required: required for `set` / `add` / `remove`; ignored for `clear`
  - Normalization: trim, lowercase, deduplicate
- `dry_run`:
  - Type: `boolean`
  - Required: no (default `false`)
  - Description: preview result only, do not write files
- `stop_on_error`:
  - Type: `boolean`
  - Required: no (default `false`)
  - Description: stop further processing when first file update fails

## Response Body

```json
{
  "status": "ok",
  "operation": "add",
  "dry_run": false,
  "summary": {
    "total": 2,
    "updated": 2,
    "unchanged": 0,
    "failed": 0,
    "skipped": 0
  },
  "results": [
    {
      "name": "claude-user-a.json",
      "status": "updated",
      "changed": true,
      "before": ["claude-3-opus"],
      "after": ["claude-3-opus", "gpt-4o*", "claude-3.7-sonnet"]
    },
    {
      "name": "gemini-user-b.json",
      "status": "updated",
      "changed": true,
      "before": [],
      "after": ["gpt-4o*", "claude-3.7-sonnet"]
    }
  ]
}
```

### Response fields

- `status`:
  - `ok`: all processed without failures
  - `partial`: some failed
  - `dry_run`: preview mode
- `operation`: normalized operation value
- `dry_run`: whether this call was preview-only
- `summary`:
  - `total`: total deduplicated input names
  - `updated`: files changed (or would change in dry-run)
  - `unchanged`: no effective change
  - `failed`: failed to process
  - `skipped`: skipped due to `stop_on_error=true`
- `results[]` item:
  - `name`: file name
  - `status`: `updated` / `unchanged` / `failed` / `skipped` / `would_update`
  - `changed`: whether before/after differs
  - `before`: normalized old excluded list
  - `after`: normalized new excluded list
  - `error`: only present when failed

## Error Responses

- `400 Bad Request`
  - invalid body
  - invalid operation
  - empty names
  - invalid file name
  - missing models for non-`clear` operation
- `500 Internal Server Error`
  - auth dir missing
  - file read/write/register unexpected errors

## Backend Processing Rules

1. Validate and deduplicate `names` (preserve first occurrence order).
2. For each file:
   - read JSON file
   - parse current excluded list from `excluded_models` or legacy `excluded-models`
   - compute new list by `operation`
   - normalize (trim/lowercase/deduplicate)
   - if changed and not `dry_run`:
     - write back as `excluded_models`
     - remove legacy `excluded-models`
     - call register path to refresh runtime auth immediately
3. Aggregate per-file results and summary.

## Frontend Integration Notes

1. Use `/v0/management/auth-files` for list/filter/select-all.
2. Submit selected names to this batch API.
3. For safety UX:
   - first call with `dry_run=true` and show before/after preview
   - confirm, then call `dry_run=false`
4. On `status=partial`, show per-file failures and provide retry for failed items only.

