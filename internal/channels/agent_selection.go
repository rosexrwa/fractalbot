package channels

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var agentNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_-]*$`)

var errDefaultAgentMissing = errors.New("default agent is not configured")
var noAgentsConfiguredMessage = "⚠️ No agents configured.\nSet agents.ohMyCode.defaultAgent or agents.ohMyCode.allowedAgents.\nTry: /agent <name> <task> (or /to <name> <task>)."

const AdminAgentName = "admin"

// AgentSelection describes the resolved target agent and task text.
type AgentSelection struct {
	Agent     string
	Task      string
	Specified bool
}

// AgentAllowlist enforces allowed agent names.
type AgentAllowlist struct {
	configured bool
	allowed    map[string]struct{}
}

// NewAgentAllowlist builds an allowlist from configured names.
func NewAgentAllowlist(names []string) AgentAllowlist {
	allowed := make(map[string]struct{})
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		allowed[trimmed] = struct{}{}
	}
	return AgentAllowlist{configured: len(allowed) > 0, allowed: allowed}
}

// ParseAgentSelection extracts a target agent and task from chat text.
// Supported syntax: /agent <name> <task...>, /to <name> <task...>,
// or /admin <task...> (routes to the reserved admin agent).
func ParseAgentSelection(text string) (AgentSelection, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return AgentSelection{Task: ""}, nil
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return AgentSelection{Task: ""}, nil
	}

	if selection, ok, err := ParseAdminSelection(trimmed); ok || err != nil {
		return selection, err
	}

	first := fields[0]
	if strings.HasPrefix(first, "/agent") || strings.HasPrefix(first, "/to") {
		command := normalizedCommandName(first)
		if command != "/agent" && command != "/to" {
			return AgentSelection{Task: trimmed}, nil
		}
		if len(fields) < 3 {
			return AgentSelection{}, fmt.Errorf("usage: %s <name> <task>", command)
		}
		return AgentSelection{
			Agent:     fields[1],
			Task:      strings.Join(fields[2:], " "),
			Specified: true,
		}, nil
	}

	return AgentSelection{Task: trimmed}, nil
}

// ParseAdminSelection parses /admin <task...>. It returns ok=false when text
// is not the admin command, and ok=true with an error for malformed admin usage.
func ParseAdminSelection(text string) (AgentSelection, bool, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return AgentSelection{}, false, nil
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return AgentSelection{}, false, nil
	}
	command := normalizedCommandName(fields[0])
	if command != "/admin" {
		return AgentSelection{}, false, nil
	}
	if len(fields) < 2 {
		return AgentSelection{}, true, fmt.Errorf("usage: /admin <text>")
	}
	return AgentSelection{
		Agent:     AdminAgentName,
		Task:      strings.Join(fields[1:], " "),
		Specified: true,
	}, true, nil
}

func normalizedCommandName(command string) string {
	if idx := strings.IndexByte(command, '@'); idx != -1 {
		command = command[:idx]
	}
	return strings.ToLower(command)
}

func agentCommandUsage(command string) string {
	if command == "/admin" {
		return "usage: /admin <text>"
	}
	if command == "" {
		command = "/agent"
	}
	return fmt.Sprintf("usage: %s <name> <task...>", command)
}

func isIncompleteAgentCommand(text string) bool {
	fields := strings.Fields(strings.TrimSpace(text))
	switch agentCommandName(text) {
	case "/admin":
		return len(fields) < 2
	case "/agent", "/to":
		return len(fields) < 3
	default:
		return false
	}
}

// ResolveAgentSelection applies defaults and allowlist validation.
func ResolveAgentSelection(selection AgentSelection, defaultAgent string, allowlist AgentAllowlist) (AgentSelection, error) {
	agent := strings.TrimSpace(selection.Agent)
	if !selection.Specified {
		agent = strings.TrimSpace(defaultAgent)
		if agent == "" {
			return AgentSelection{}, errDefaultAgentMissing
		}
	}

	if err := ValidateAgentName(agent); err != nil {
		return AgentSelection{}, err
	}

	if err := allowlist.Validate(agent, defaultAgent); err != nil {
		return AgentSelection{}, err
	}

	selection.Agent = agent
	return selection, nil
}

func agentCommandName(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return ""
	}
	command := normalizedCommandName(fields[0])
	if command != "/agent" && command != "/to" && command != "/admin" {
		return ""
	}
	return command
}

// ValidateAgentName enforces allowed agent naming.
func ValidateAgentName(name string) error {
	if !agentNamePattern.MatchString(name) {
		return fmt.Errorf("invalid agent name %q", name)
	}
	return nil
}

// Validate ensures the agent is allowed per configuration.
func (a AgentAllowlist) Validate(agentName, defaultAgent string) error {
	if a.configured {
		if _, ok := a.allowed[agentName]; !ok {
			return fmt.Errorf("agent %q is not allowed", agentName)
		}
		return nil
	}

	defaultName := strings.TrimSpace(defaultAgent)
	if defaultName == "" {
		return errDefaultAgentMissing
	}
	if agentName != defaultName {
		return fmt.Errorf("agent %q is not allowed", agentName)
	}
	return nil
}

// Names returns the configured allowlist names in sorted order.
func (a AgentAllowlist) Names() []string {
	if !a.configured {
		return nil
	}
	names := make([]string, 0, len(a.allowed))
	for name := range a.allowed {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func filterOutAgentName(names []string, target string) []string {
	if target == "" {
		return names
	}
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if name == target {
			continue
		}
		filtered = append(filtered, name)
	}
	return filtered
}

func agentNotAllowedMessage(err error, defaultAgent string, allowlist AgentAllowlist) string {
	message := fmt.Sprintf("❌ %v", err)
	if allowlist.configured {
		return fmt.Sprintf("%s\nTip: add to agents.ohMyCode.allowedAgents or use /agents.", message)
	}
	if strings.TrimSpace(defaultAgent) != "" {
		return fmt.Sprintf("%s\nOnly the default agent is enabled.\nTip: configure agents.ohMyCode.allowedAgents to allow others, or use /agents.", message)
	}
	return fmt.Sprintf("%s\nTip: configure agents.ohMyCode.defaultAgent or agents.ohMyCode.allowedAgents.", message)
}
