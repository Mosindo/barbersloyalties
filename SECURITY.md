# Security Policy

## Scope
This repository is public. Never commit real secrets, credentials, private keys, or production connection strings.

## Allowed patterns
- `.env.example` and templates with fake placeholders only
- Test-only secrets marked as `dev-only-change-this`

## Forbidden data in git
- Real API keys (Stripe, cloud providers, third-party services)
- Private keys (`.pem`, `.key`, SSH private keys)
- Production passwords or connection strings
- Tokens (GitHub PAT, JWT signing keys used in production)

## Pre-push checklist
1. Confirm no `.env` file is staged.
2. Confirm no private key/cert file is staged.
3. Confirm placeholders remain fake (`dev-only-change-this`).
4. Run local grep checks:
   - `git grep -nI -E "sk_live_|ghp_|github_pat_|AKIA|BEGIN .* PRIVATE KEY"`
5. Verify CI secret scan passes on PR.

## Incident response runbook
If a secret is exposed (current commit or history):
1. Rotate/revoke the secret immediately at provider side.
2. Remove secret from code and configuration templates.
3. Purge git history if needed (`git filter-repo`) and force-push with team coordination.
4. Invalidate sessions/tokens impacted by the leak.
5. Document incident timeline and remediation in `docs/`.

## Disclosure
Report security concerns privately to repository maintainers before public disclosure.
