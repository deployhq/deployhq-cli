package auth

import "fmt"

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

// SignInHint is the guidance shown whenever dhq needs credentials it cannot
// prompt for (for example when run by an AI agent). The API key is a secret:
// the person enters it themselves, in a terminal the agent does not drive, so
// it never ends up in a chat transcript.
func SignInHint(account, host string) string {
	return "Ask the person to sign in themselves: run 'dhq auth login' in a separate terminal window " +
		"(not through the AI agent). It prompts for the account, email and API key, and stores them securely.\n" +
		"The API key is under " + APIKeyLocation(account, host) + ".\n" +
		"Never paste an API key into an AI chat.\n" +
		"For CI, set DEPLOYHQ_ACCOUNT, DEPLOYHQ_EMAIL and DEPLOYHQ_API_KEY as secret environment variables instead."
}
