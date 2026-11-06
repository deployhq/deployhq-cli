package auth

import (
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/harness"
	"github.com/deployhq/deployhq-cli/internal/skillinstaller"
)

// APIKeyLocation tells the person where to find their API key in the DeployHQ
// web app. account and host are optional; when account is empty a placeholder
// is used in the URL.
func APIKeyLocation(account, host string) string {
	if host == "" {
		host = "deployhq.com"
	}
	if account == "" {
		account = "<your-account>"
	}
	return fmt.Sprintf("Settings → Security → API Credentials in DeployHQ (https://%s.%s/security)", account, host)
}

// SeparateTerminal says where the person runs a sign-in or sign-up command:
// in a terminal the agent does not drive. Agents read "not through the AI
// agent" as allowing their own '!' shell shortcut (Claude Code even suggests
// it for interactive logins), but a command run that way gets no terminal on
// stdin or stdout, so dhq can't prompt and refuses. Hence named explicitly,
// with the reason.
const SeparateTerminal = "in a separate terminal window (not through the AI agent, and not with its '!' shortcut, " +
	"which gives dhq no terminal to prompt in)"

// SkillInstallCommand is the command that installs DeployHQ's skill for the
// agent running dhq only (see skillinstaller.InstallCommandFor), never for
// every AI tool on the machine.
func SkillInstallCommand() string {
	return skillinstaller.InstallCommandFor(harness.Detect().Name)
}

// SkillPointer sends an agent to DeployHQ's skill. Agents act on the CLI's
// errors but rarely run 'dhq --help', so the messages they meet first (not
// signed in, no project) carry it.
func SkillPointer() string {
	cmd := SkillInstallCommand()
	names := ""
	if strings.Contains(cmd, "<") {
		names = " ('dhq skills list' names the agents)"
	}
	return "AI agents: install DeployHQ's skill for yourself with '" + cmd + "'" + names +
		", then read and follow it: it has the setup steps, including when to ask the person."
}

// SignInHint is the guidance shown whenever dhq needs credentials it cannot
// prompt for (for example when run by an AI agent). The API key is a secret:
// the person enters it themselves, in a terminal the agent does not drive, so
// it never ends up in a chat transcript. A person without an account signs up
// the same way, since sign-up asks for a password.
func SignInHint(account, host string) string {
	return "Ask the person to sign in themselves: run 'dhq auth login' " + SeparateTerminal +
		". It prompts for the account, email and API key, and stores them securely.\n" +
		"If the person has no DeployHQ account yet, they run 'dhq signup' themselves instead, the same way: " + SeparateTerminal + ".\n" +
		"The API key is under " + APIKeyLocation(account, host) + ".\n" +
		"Never paste an API key into an AI chat.\n" +
		"For CI, set DEPLOYHQ_ACCOUNT, DEPLOYHQ_EMAIL and DEPLOYHQ_API_KEY as secret environment variables instead.\n" +
		SkillPointer()
}
