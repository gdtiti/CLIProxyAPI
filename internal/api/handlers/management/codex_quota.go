package management

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/codexquota"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

type codexPayloadConfigResponse struct {
	Default     []config.PayloadRule       `json:"default"`
	DefaultRaw  []config.PayloadRule       `json:"default_raw"`
	Override    []config.PayloadRule       `json:"override"`
	OverrideRaw []config.PayloadRule       `json:"override_raw"`
	Filter      []config.PayloadFilterRule `json:"filter"`
}

type codexHeaderDefaultsResponse struct {
	UserAgent    string `json:"user_agent"`
	BetaFeatures string `json:"beta_features"`
}

type codexAuthConfigResponse struct {
	CodexHeaderDefaults codexHeaderDefaultsResponse `json:"codex_header_defaults"`
	Payload             codexPayloadConfigResponse  `json:"payload"`
	Notes               map[string]any              `json:"notes"`
}

type codexAuthConfigRequest struct {
	CodexHeaderDefaults codexHeaderDefaultsResponse `json:"codex_header_defaults"`
	Payload             codexPayloadConfigResponse  `json:"payload"`
}

func (h *Handler) GetCodexAuthQuota(c *gin.Context) {
	service := codexquota.DefaultService()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "codex quota service unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": service.ListSnapshots()})
}

func (h *Handler) GetCodexAuthQuotaByIndex(c *gin.Context) {
	service := codexquota.DefaultService()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "codex quota service unavailable"})
		return
	}
	authIndex := strings.TrimSpace(c.Param("auth_index"))
	if authIndex == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing auth_index"})
		return
	}
	snapshot, ok := service.GetSnapshot(authIndex)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "codex auth not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"snapshot": snapshot,
		"events":   service.ListEvents(authIndex, 50),
	})
}

func (h *Handler) GetCodexAuthEvents(c *gin.Context) {
	service := codexquota.DefaultService()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "codex quota service unavailable"})
		return
	}
	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = value
	}
	authIndex := strings.TrimSpace(c.Query("auth_index"))
	c.JSON(http.StatusOK, gin.H{"events": service.ListEvents(authIndex, limit)})
}

func (h *Handler) GetCodexAuthUsage(c *gin.Context) {
	service := codexquota.DefaultService()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "codex quota service unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"usage": service.ListRollups()})
}

func (h *Handler) GetCodexAuthConfig(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config unavailable"})
		return
	}
	c.JSON(http.StatusOK, codexAuthConfigResponse{
		CodexHeaderDefaults: codexHeaderDefaultsResponse{
			UserAgent:    h.cfg.CodexHeaderDefaults.UserAgent,
			BetaFeatures: h.cfg.CodexHeaderDefaults.BetaFeatures,
		},
		Payload: codexPayloadConfigResponse{
			Default:     filterCodexPayloadRules(h.cfg.Payload.Default),
			DefaultRaw:  filterCodexPayloadRules(h.cfg.Payload.DefaultRaw),
			Override:    filterCodexPayloadRules(h.cfg.Payload.Override),
			OverrideRaw: filterCodexPayloadRules(h.cfg.Payload.OverrideRaw),
			Filter:      filterCodexPayloadFilterRules(h.cfg.Payload.Filter),
		},
		Notes: map[string]any{
			"custom_params":              "Use payload rules to add Codex-specific request fields such as instructions or custom context controls.",
			"one_million_context":        "This repository does not expose a dedicated 1m context field. Configure it through Codex payload rules if your upstream accepts it.",
			"recovered_tokens_available": false,
		},
	})
}

func (h *Handler) PutCodexAuthConfig(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config unavailable"})
		return
	}
	if strings.TrimSpace(h.configFilePath) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config file path unavailable"})
		return
	}

	var body codexAuthConfigRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body", "message": err.Error()})
		return
	}
	if err := validateCodexPayloadRequest(body.Payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload rules", "message": err.Error()})
		return
	}

	headerDefaults := config.CodexHeaderDefaults{
		UserAgent:    strings.TrimSpace(body.CodexHeaderDefaults.UserAgent),
		BetaFeatures: strings.TrimSpace(body.CodexHeaderDefaults.BetaFeatures),
	}

	h.mu.Lock()
	h.cfg.CodexHeaderDefaults = headerDefaults
	h.cfg.Payload.Default = replaceCodexPayloadRules(h.cfg.Payload.Default, body.Payload.Default)
	h.cfg.Payload.DefaultRaw = replaceCodexPayloadRules(h.cfg.Payload.DefaultRaw, body.Payload.DefaultRaw)
	h.cfg.Payload.Override = replaceCodexPayloadRules(h.cfg.Payload.Override, body.Payload.Override)
	h.cfg.Payload.OverrideRaw = replaceCodexPayloadRules(h.cfg.Payload.OverrideRaw, body.Payload.OverrideRaw)
	h.cfg.Payload.Filter = replaceCodexPayloadFilterRules(h.cfg.Payload.Filter, body.Payload.Filter)
	h.cfg.SanitizeCodexHeaderDefaults()
	h.cfg.SanitizePayloadRules()
	h.mu.Unlock()

	h.persist(c)
}

func (h *Handler) ServeCodexAuthPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(codexAuthManagementPage))
}

func filterCodexPayloadRules(rules []config.PayloadRule) []config.PayloadRule {
	filtered := make([]config.PayloadRule, 0, len(rules))
	for _, rule := range rules {
		if isPureCodexPayloadRule(rule.Models) {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

func filterCodexPayloadFilterRules(rules []config.PayloadFilterRule) []config.PayloadFilterRule {
	filtered := make([]config.PayloadFilterRule, 0, len(rules))
	for _, rule := range rules {
		if isPureCodexPayloadRule(rule.Models) {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

func replaceCodexPayloadRules(existing []config.PayloadRule, incoming []config.PayloadRule) []config.PayloadRule {
	out := make([]config.PayloadRule, 0, len(existing)+len(incoming))
	for _, rule := range existing {
		if !isPureCodexPayloadRule(rule.Models) {
			out = append(out, rule)
		}
	}
	out = append(out, incoming...)
	return out
}

func replaceCodexPayloadFilterRules(existing []config.PayloadFilterRule, incoming []config.PayloadFilterRule) []config.PayloadFilterRule {
	out := make([]config.PayloadFilterRule, 0, len(existing)+len(incoming))
	for _, rule := range existing {
		if !isPureCodexPayloadRule(rule.Models) {
			out = append(out, rule)
		}
	}
	out = append(out, incoming...)
	return out
}

func validateCodexPayloadRequest(payload codexPayloadConfigResponse) error {
	for _, rule := range slices.Concat(payload.Default, payload.DefaultRaw, payload.Override, payload.OverrideRaw) {
		if !isPureCodexPayloadRule(rule.Models) {
			return errInvalidCodexRule("payload rule must contain protocol=codex for every model")
		}
	}
	for _, rule := range payload.Filter {
		if !isPureCodexPayloadRule(rule.Models) {
			return errInvalidCodexRule("payload filter rule must contain protocol=codex for every model")
		}
	}
	for _, rule := range slices.Concat(payload.DefaultRaw, payload.OverrideRaw) {
		for _, value := range rule.Params {
			raw, ok := value.(string)
			if !ok {
				continue
			}
			if !json.Valid([]byte(strings.TrimSpace(raw))) {
				return errInvalidCodexRule("raw payload values must be valid JSON strings")
			}
		}
	}
	return nil
}

func isPureCodexPayloadRule(models []config.PayloadModelRule) bool {
	if len(models) == 0 {
		return false
	}
	for _, model := range models {
		if !strings.EqualFold(strings.TrimSpace(model.Protocol), "codex") {
			return false
		}
	}
	return true
}

type invalidCodexRuleError string

func (e invalidCodexRuleError) Error() string { return string(e) }

func errInvalidCodexRule(message string) error {
	return invalidCodexRuleError(message)
}

const codexAuthManagementPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Codex Auth Center</title>
  <style>
    :root {
      color-scheme: light dark;
      font-family: Arial, sans-serif;
    }
    body {
      margin: 0;
      background: #0f172a;
      color: #e2e8f0;
    }
    .wrap {
      max-width: 1400px;
      margin: 0 auto;
      padding: 20px;
    }
    .toolbar, .grid {
      display: grid;
      gap: 16px;
    }
    .toolbar {
      grid-template-columns: 240px 160px 160px 1fr;
      align-items: center;
      margin-bottom: 16px;
    }
    .grid {
      grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    }
    .panel {
      background: rgba(15, 23, 42, 0.92);
      border: 1px solid #334155;
      border-radius: 10px;
      padding: 16px;
      box-shadow: 0 10px 30px rgba(15, 23, 42, 0.25);
    }
    h1, h2 {
      margin-top: 0;
    }
    label {
      display: block;
      font-size: 13px;
      margin-bottom: 6px;
      color: #cbd5e1;
    }
    input, textarea, button {
      width: 100%;
      box-sizing: border-box;
      border-radius: 8px;
      border: 1px solid #475569;
      background: #020617;
      color: #e2e8f0;
      padding: 10px 12px;
      font-size: 14px;
    }
    textarea {
      min-height: 120px;
      resize: vertical;
      font-family: Consolas, monospace;
    }
    button {
      cursor: pointer;
      background: #1d4ed8;
      border-color: #1d4ed8;
      font-weight: 600;
    }
    button.secondary {
      background: #334155;
      border-color: #334155;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 13px;
    }
    th, td {
      border-bottom: 1px solid #334155;
      padding: 8px 6px;
      text-align: left;
      vertical-align: top;
    }
    code, pre {
      font-family: Consolas, monospace;
      white-space: pre-wrap;
      word-break: break-word;
    }
    .note {
      color: #93c5fd;
      font-size: 13px;
      margin-bottom: 12px;
    }
    .status {
      min-height: 24px;
      color: #facc15;
      font-size: 13px;
    }
    .mono {
      font-family: Consolas, monospace;
    }
  </style>
</head>
<body>
  <div class="wrap">
    <h1>Codex Auth Center</h1>
    <div class="toolbar">
      <div>
        <label for="key">Management key</label>
        <input id="key" type="password" placeholder="X-Management-Key">
      </div>
      <div>
        <label>&nbsp;</label>
        <button onclick="refreshAll()">Refresh all</button>
      </div>
      <div>
        <label>&nbsp;</label>
        <button class="secondary" onclick="saveConfig()">Save config</button>
      </div>
      <div class="status" id="status"></div>
    </div>

    <div class="grid">
      <section class="panel">
        <h2>Codex config</h2>
        <div class="note">There is no dedicated 1m context field in this repo. Use payload rules for custom Codex parameters such as instructions and upstream-specific context controls.</div>
        <label for="userAgent">codex-header-defaults.user-agent</label>
        <input id="userAgent" type="text">
        <label for="betaFeatures" style="margin-top:12px;">codex-header-defaults.beta-features</label>
        <input id="betaFeatures" type="text">
        <label for="payloadDefault" style="margin-top:12px;">payload.default (protocol=codex only)</label>
        <textarea id="payloadDefault"></textarea>
        <label for="payloadDefaultRaw">payload.default_raw</label>
        <textarea id="payloadDefaultRaw"></textarea>
        <label for="payloadOverride">payload.override</label>
        <textarea id="payloadOverride"></textarea>
        <label for="payloadOverrideRaw">payload.override_raw</label>
        <textarea id="payloadOverrideRaw"></textarea>
        <label for="payloadFilter">payload.filter</label>
        <textarea id="payloadFilter"></textarea>
      </section>

      <section class="panel">
        <h2>Codex accounts</h2>
        <div class="note">Recovered tokens are not provided by the upstream contract in this repo, so the field stays empty.</div>
        <div style="overflow:auto;">
          <table id="accountsTable">
            <thead>
              <tr>
                <th>Auth index</th>
                <th>Account</th>
                <th>File</th>
                <th>Status</th>
                <th>Quota</th>
                <th>Recover</th>
                <th>Requests</th>
                <th>Avg total</th>
              </tr>
            </thead>
            <tbody></tbody>
          </table>
        </div>
      </section>

      <section class="panel">
        <h2>Usage rollups</h2>
        <pre id="usageView">[]</pre>
      </section>

      <section class="panel">
        <h2>Events</h2>
        <div style="display:grid; grid-template-columns: 1fr 160px; gap:12px; margin-bottom:12px;">
          <input id="eventAuthIndex" type="text" placeholder="Optional auth_index filter">
          <button onclick="loadEvents()">Load events</button>
        </div>
        <pre id="eventsView">[]</pre>
      </section>

      <section class="panel">
        <h2>Account detail</h2>
        <div style="display:grid; grid-template-columns: 1fr 160px; gap:12px; margin-bottom:12px;">
          <input id="detailAuthIndex" type="text" placeholder="Auth index">
          <button onclick="loadDetail()">Load detail</button>
        </div>
        <pre id="detailView">{}</pre>
      </section>
    </div>
  </div>

  <script>
    function setStatus(message, isError) {
      var el = document.getElementById('status');
      el.textContent = message || '';
      el.style.color = isError ? '#fca5a5' : '#93c5fd';
    }

    function keyHeaders() {
      var key = document.getElementById('key').value.trim();
      var headers = { 'Content-Type': 'application/json' };
      if (key) {
        headers['X-Management-Key'] = key;
      }
      return headers;
    }

    async function api(path, options) {
      var response = await fetch(path, Object.assign({ headers: keyHeaders() }, options || {}));
      if (!response.ok) {
        var text = await response.text();
        throw new Error(response.status + ': ' + text);
      }
      if (response.status === 204) {
        return null;
      }
      return response.json();
    }

    function pretty(value) {
      return JSON.stringify(value == null ? null : value, null, 2);
    }

    async function refreshAll() {
      setStatus('Loading...', false);
      try {
        await Promise.all([loadConfig(), loadAccounts(), loadUsage(), loadEvents()]);
        setStatus('Refreshed at ' + new Date().toLocaleString(), false);
      } catch (error) {
        setStatus(error.message, true);
      }
    }

    async function loadConfig() {
      var data = await api('/v0/management/codex-auth-config');
      document.getElementById('userAgent').value = data.codex_header_defaults.user_agent || '';
      document.getElementById('betaFeatures').value = data.codex_header_defaults.beta_features || '';
      document.getElementById('payloadDefault').value = pretty(data.payload.default || []);
      document.getElementById('payloadDefaultRaw').value = pretty(data.payload.default_raw || []);
      document.getElementById('payloadOverride').value = pretty(data.payload.override || []);
      document.getElementById('payloadOverrideRaw').value = pretty(data.payload.override_raw || []);
      document.getElementById('payloadFilter').value = pretty(data.payload.filter || []);
    }

    async function saveConfig() {
      try {
        var body = {
          codex_header_defaults: {
            user_agent: document.getElementById('userAgent').value,
            beta_features: document.getElementById('betaFeatures').value
          },
          payload: {
            default: JSON.parse(document.getElementById('payloadDefault').value || '[]'),
            default_raw: JSON.parse(document.getElementById('payloadDefaultRaw').value || '[]'),
            override: JSON.parse(document.getElementById('payloadOverride').value || '[]'),
            override_raw: JSON.parse(document.getElementById('payloadOverrideRaw').value || '[]'),
            filter: JSON.parse(document.getElementById('payloadFilter').value || '[]')
          }
        };
        await api('/v0/management/codex-auth-config', { method: 'PUT', body: JSON.stringify(body) });
        setStatus('Config saved', false);
      } catch (error) {
        setStatus(error.message, true);
      }
    }

    async function loadAccounts() {
      var data = await api('/v0/management/codex-auth-quota');
      var tbody = document.querySelector('#accountsTable tbody');
      tbody.innerHTML = '';
      (data.accounts || []).forEach(function (item) {
        var tr = document.createElement('tr');
        tr.innerHTML =
          '<td class="mono"><button class="secondary" style="width:auto;padding:4px 8px;" onclick="selectDetail(\'' + escapeHtml(item.auth_index || '') + '\')">' + escapeHtml(item.auth_index || '') + '</button></td>' +
          '<td>' + escapeHtml(item.account || '') + '</td>' +
          '<td class="mono">' + escapeHtml(item.file_name || '') + '</td>' +
          '<td>' + escapeHtml(item.status || '') + (item.disabled ? ' / disabled' : '') + '</td>' +
          '<td>' + (item.quota_exceeded ? 'yes' : 'no') + '</td>' +
          '<td>' + escapeHtml(item.next_recover_at || '') + '</td>' +
          '<td>' + escapeHtml(String((item.usage || {}).request_count || 0)) + '</td>' +
          '<td>' + escapeHtml(String((item.usage || {}).avg_total_tokens || 0)) + '</td>';
        tbody.appendChild(tr);
      });
    }

    async function loadUsage() {
      var data = await api('/v0/management/codex-auth-usage');
      document.getElementById('usageView').textContent = pretty(data.usage || []);
    }

    async function loadEvents() {
      var authIndex = document.getElementById('eventAuthIndex').value.trim();
      var path = '/v0/management/codex-auth-events?limit=100';
      if (authIndex) {
        path += '&auth_index=' + encodeURIComponent(authIndex);
      }
      var data = await api(path);
      document.getElementById('eventsView').textContent = pretty(data.events || []);
    }

    function selectDetail(authIndex) {
      document.getElementById('detailAuthIndex').value = authIndex;
      loadDetail();
    }

    async function loadDetail() {
      var authIndex = document.getElementById('detailAuthIndex').value.trim();
      if (!authIndex) {
        setStatus('auth_index is required for detail', true);
        return;
      }
      try {
        var data = await api('/v0/management/codex-auth-quota/' + encodeURIComponent(authIndex));
        document.getElementById('detailView').textContent = pretty(data);
        setStatus('Loaded detail for ' + authIndex, false);
      } catch (error) {
        setStatus(error.message, true);
      }
    }

    function escapeHtml(value) {
      return String(value)
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
    }
  </script>
</body>
</html>`
