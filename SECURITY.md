# Security Policy

## Supported versions

This project is currently in **beta**. Security fixes and patches are provided on a **best-effort** basis.

We generally support:
- the latest commit on the default branch, and/or
- the most recent tagged release (if tags exist)

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities.

Preferred reporting path:
1. Use **GitHub Security Advisories** ("Report a vulnerability") if enabled for this repository.
2. If Security Advisories are not available, contact the repository maintainers via GitHub (for example, by starting a discussion and asking for a private channel).

When reporting, include:
- A clear description of the vulnerability and impact
- Steps to reproduce
- Proof-of-concept (if available)
- Affected versions/commits
- Any suggested remediation

### Sensitive information

- Do **not** include secrets in reports (CPI service keys, tokens, private keys).
- If logs are needed, redact tokens/secrets and any confidential tenant details.

## Disclosure policy

- We aim to acknowledge valid reports as soon as possible.
- We may request additional details to reproduce the issue.
- We will coordinate disclosure timing with the reporter when feasible.

## Security best practices for users

- Never commit CPI service keys (`service-key*.json`) to Git.
- Store tokens in environment variables or secret managers.
- Review your local `.iflowkit/` directory permissions.
- Prefer least-privilege credentials for CPI and Git access.
