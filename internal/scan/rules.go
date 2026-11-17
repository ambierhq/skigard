package scan

import (
	"regexp"
	"strings"
)

type rule struct {
	ID           string
	Name         string
	Family       string
	Sev          Severity
	Caps         []string
	Msg          string
	Re           *regexp.Regexp
	DocLow       bool // in prose of markdown files, downgrade to low
	MarkdownOnly bool
	// Pre is a cheap prefilter on the lowercased line: the regex only runs if
	// at least one entry is present. An entry "a&b" needs both substrings. It
	// must never reject a line the regex would match (see TestPrefilterCoversSamples).
	Pre    []string
	MinLen int
}

func (r rule) maybe(lower string) bool {
	if len(lower) < r.MinLen {
		return false
	}
	if len(r.Pre) == 0 {
		return true
	}
	for _, k := range r.Pre {
		if !strings.Contains(k, "&") {
			if strings.Contains(lower, k) {
				return true
			}
			continue
		}
		all := true
		for _, part := range strings.Split(k, "&") {
			if !strings.Contains(lower, part) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func re(s string) *regexp.Regexp { return regexp.MustCompile(`(?i)` + s) }

var lineRules = []rule{
	{ID: "SKG-EXEC-001", Name: "pipe-to-shell", Family: "EXEC", Sev: High, Caps: []string{"remote_exec", "network_egress", "dynamic_exec"},
		Pre: []string{"curl", "wget", "fetch"},
		Msg: "downloads and pipes straight into an interpreter",
		Re:  re(`\b(curl|wget|fetch)\b[^\n|]*\|\s*(sudo\s+)?((ba|z|da|k)?sh|python3?|node|perl|ruby)\b`)},
	{ID: "SKG-EXEC-002", Name: "reverse-shell", Family: "EXEC", Sev: High, Caps: []string{"network_egress", "dynamic_exec"},
		Pre: []string{"/dev/tcp/", "bash", "mkfifo", "socat", " -e"},
		Msg: "reverse-shell pattern",
		Re:  re(`/dev/tcp/|\bnc\b[^\n]*\s-e\b|\bbash\s+-i\b|mkfifo[^\n]*\bnc\b|socat[^\n]*exec`)},
	{ID: "SKG-EXEC-003", Name: "destructive", Family: "EXEC", Sev: High, Caps: []string{"destructive_write"},
		Pre: []string{"rm ", "rm\t"},
		Msg: "recursive delete of root or home",
		Re:  re(`\brm\s+-[a-z]*r[a-z]*\s+(/|~/?|\$HOME/?)(\s|$|\*)`)},
	{ID: "SKG-EXEC-004", Name: "dynamic-exec", Family: "EXEC", Sev: Medium, Caps: []string{"dynamic_exec"}, DocLow: true,
		Pre: []string{"eval", "exec", "os.system", "subprocess", "child_process", "__import__", "new&function"},
		Msg: "dynamic code or process execution",
		Re:  re(`(^|[^.\w])(eval|exec)\s*\(|\beval\s+["'$]|\bos\.system\b|\bsubprocess\b|child_process|(?-i:\bnew\s+Function\s*\()|__import__|\bexec\.Command\b`)},
	{ID: "SKG-EXEC-005", Name: "privilege", Family: "EXEC", Sev: Medium, DocLow: true,
		Pre: []string{"sudo", "chmod", "chown"},
		Msg: "privilege or permission change",
		Re:  re(`\bsudo\b|chmod\s+(\+x|[0-7]*7[0-7]*)\b|\bchown\b`)},
	{ID: "SKG-EXEC-006", Name: "persistence", Family: "EXEC", Sev: Medium, Caps: []string{"persistence"}, DocLow: true,
		Pre: []string{".bashrc", ".zshrc", ".zprofile", ".bash_profile", ".profile", "crontab", "launchctl", "launchagents", "systemd", "systemctl"},
		Msg: "shell startup or scheduler persistence",
		Re:  re(`\.(bashrc|zshrc|zprofile|bash_profile)\b|\.profile\b|crontab|launchctl|LaunchAgents|systemd|systemctl`)},
	{ID: "SKG-SECRETS-001", Name: "credential-access", Family: "SECRETS", Sev: High, Caps: []string{"sensitive_read"},
		Pre: []string{".ssh", ".aws", ".gnupg", ".kube", ".netrc", ".npmrc", ".pypirc", "docker/config.json", "id_rsa", "id_ed25519", "credentials.json", "find-generic-password", "login data", ".config/gcloud", "keychain"},
		Msg: "touches credential or secret locations",
		Re:  re(`\.(ssh|aws|gnupg|kube|netrc|npmrc|pypirc)\b|docker/config\.json|id_rsa|id_ed25519|credentials\.json|find-generic-password|Login Data|\.config/gcloud|\bkeychain`)},
	{ID: "SKG-SECRETS-002", Name: "env-access", Family: "SECRETS", Sev: Medium, Caps: []string{"env_read"}, DocLow: true,
		Pre: []string{"os.environ", "process.env", "os.getenv", "printenv", "$", ".env"},
		Msg: "reads environment or secrets",
		Re:  re(`os\.environ|process\.env|os\.Getenv|\bprintenv\b|\$\{?[A-Z_]*(KEY|TOKEN|SECRET|PASSWORD)\b|(^|[\s/])\.env\b`)},
	{ID: "SKG-SECRETS-003", Name: "exfiltration", Family: "SECRETS", Sev: High, Caps: []string{"network_egress", "sensitive_read"},
		Pre: []string{"curl", "wget"},
		Msg: "uploads secrets or environment over the network",
		Re:  re(`\b(curl|wget)\b[^\n]*(-d|--data[a-z-]*|-F|--upload-file|-T)\s*@?[^\n]*(\.env\b|id_rsa|credentials|printenv|\$\(?env\b)`)},
	{ID: "SKG-NET-001", Name: "network-tool", Family: "NET", Sev: Medium, Caps: []string{"network_egress"}, DocLow: true,
		Pre: []string{"curl", "wget", "ncat", "ssh", "scp", "rsync", "ftp", " nc ", "\tnc ", "|nc", "(nc", ";nc", "`nc"},
		Msg: "network tool",
		Re:  re(`\b(curl|wget|nc|ncat|ssh|scp|rsync|ftp)\b`)},
	{ID: "SKG-NET-002", Name: "network-lib", Family: "NET", Sev: Medium, Caps: []string{"network_egress"}, DocLow: true,
		Pre: []string{"requests.", "urllib", "http.client", "httpx", "aiohttp", "fetch(", "xmlhttprequest", "websocket", "socket.", "net/http", "http.get", "http.post"},
		Msg: "network library",
		Re:  re(`\b(requests\.(get|post|put|request)|urllib|http\.client|httpx|aiohttp|fetch\(|XMLHttpRequest|WebSocket|socket\.|net/http|http\.(Get|Post))`)},
	{ID: "SKG-PROMPT-001", Name: "instruction-override", Family: "PROMPT", Sev: High,
		Pre: []string{"ignore", "disregard", "you are now", "override"},
		Msg: "tries to override higher-priority instructions",
		Re:  re(`ignore\s+(all\s+|any\s+)?(the\s+)?(previous|prior|above|earlier|system)\s+(instructions|prompts?|rules)|disregard\s+[^\n]{0,40}(instructions|rules|system prompt)|you\s+are\s+now\s+(in\s+)?(dan|developer mode|unrestricted)|(system|developer)\s+(prompt|message)\s+override`)},
	{ID: "SKG-PROMPT-002", Name: "concealment", Family: "PROMPT", Sev: High,
		Pre: []string{"tell", "inform", "mention", "reveal", "show", "notify", "telling", "notifying", "asking", "silently", "secretly", "covertly", "quietly"},
		Msg: "asks the agent to hide actions from the user",
		Re:  re(`(do\s+not|don'?t|never)\s+(tell|inform|mention|reveal|show|notify)[^\n]{0,40}\buser\b|without\s+(telling|informing|notifying|asking)\s+(the\s+)?user|(silently|secretly|covertly|quietly)\s+(run|execute|send|upload|post|install|download)`)},
	{ID: "SKG-PROMPT-003", Name: "safety-bypass", Family: "PROMPT", Sev: Medium,
		Pre: []string{"disable", "bypass", "skip", "--yolo"},
		Msg: "asks to bypass safety controls",
		Re:  re(`(disable|bypass|skip)\s+(the\s+)?(permission|approval|confirmation|safety|sandbox)|dangerously-skip-permissions|--yolo`)},
	{ID: "SKG-OBF-002", Name: "base64-decode", Family: "OBF", Sev: Medium, DocLow: true,
		Pre: []string{"base64", "b64decode", "atob"},
		Msg: "decodes base64 (possible obfuscation)",
		Re:  re(`base64\s+(-d|--decode|-D)|b64decode|atob\(|Buffer\.from\([^)]*base64|base64\.StdEncoding\.Decode`)},
	{ID: "SKG-OBF-003", Name: "encoded-blob", Family: "OBF", Sev: Medium,
		MinLen: 120,
		Msg:    "long encoded-looking blob",
		Re:     regexp.MustCompile(`[A-Za-z0-9+/]{120,}={0,2}`)},
	{ID: "SKG-OBF-004", Name: "html-comment", Family: "OBF", Sev: Medium, MarkdownOnly: true,
		Pre: []string{"<!--"},
		Msg: "HTML comment (can hide instructions)",
		Re:  regexp.MustCompile(`<!--`)},
	{ID: "SKG-SUPPLY-001", Name: "package-install", Family: "SUPPLY", Sev: Medium, Caps: []string{"supply_chain"}, DocLow: true,
		Pre: []string{"npm", "pnpm", "yarn", "pip", "brew", "gem", "cargo", "npx", "uvx", "uv ", "go "},
		Msg: "installs third-party packages",
		Re:  re(`\b(npm|pnpm|yarn|pip3?|uv|brew|gem|cargo|go)\s+(install|add|i|get)\b|\bnpx\s|\buvx\s|\bpipx\b`)},
	{ID: "SKG-SUPPLY-002", Name: "remote-instructions", Family: "SUPPLY", Sev: Medium, Caps: []string{"external_instruction"},
		Pre: []string{"instruction&http", "prompt&http", "skill&http", "rules&http"},
		Msg: "loads instructions from a remote URL",
		Re:  re(`\b(fetch|download|curl|wget|read|load|follow)\b[^\n]{0,40}\b(instructions?|prompt|skill|rules)\b[^\n]{0,60}https?://`)},
}

// Rules that are not line patterns, kept here so every ID lives in one place.
var structural = map[string]struct {
	Name, Family string
	Sev          Severity
}{
	"SKG-OBF-001":      {"invisible-unicode", "OBF", High},
	"SKG-OBF-005":      {"nul-bytes", "OBF", Medium},
	"SKG-MANIFEST-001": {"invalid-skill", "MANIFEST", High},
	"SKG-MANIFEST-002": {"broad-tools", "MANIFEST", Medium},
	"SKG-FS-001":       {"binary-file", "FS", High},
	"SKG-FS-002":       {"executable-binary", "FS", High},
	"SKG-FS-003":       {"large-file", "FS", Medium},
	"SKG-FS-004":       {"executable-file", "FS", Medium},
	"SKG-FS-005":       {"script-present", "FS", Low},
	"SKG-FS-006":       {"asset-not-inspected", "FS", Info},
	"SKG-FS-007":       {"symlink", "FS", High},
	"SKG-FS-008":       {"too-many-files", "FS", High},
	"SKG-NET-003":      {"external-url", "NET", Low},
	"SKG-CHAIN-001":    {"exfiltration-chain", "CHAIN", Critical},
	"SKG-CHAIN-002":    {"persistent-exec", "CHAIN", High},
}

// Catalog returns every rule (id, name, family, severity, message) for docs.
func Catalog() [][5]string {
	var out [][5]string
	for _, r := range lineRules {
		msg := r.Msg
		if r.DocLow {
			msg += " (prose-low)"
		}
		out = append(out, [5]string{r.ID, r.Name, r.Family, string(r.Sev), msg})
	}
	for id, s := range structural {
		out = append(out, [5]string{id, s.Name, s.Family, string(s.Sev), ""})
	}
	return out
}
