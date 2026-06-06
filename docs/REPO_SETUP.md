# Repo Setup — Recommended Settings

Apply these settings via GitHub UI or `gh` CLI after the repo is created.

## Branch protection (main)

```bash
gh api repos/fox-in-the-box-ai/fox-fleet/branches/main/protection -X PUT --input - <<'EOF'
{
  "required_status_checks": {
    "strict": true,
    "contexts": ["Lint", "Test", "Build (ubuntu-latest)"]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": {
    "required_approving_review_count": 1
  },
  "restrictions": null
}
EOF
```

## Security settings

```bash
gh api repos/fox-in-the-box-ai/fox-fleet -X PATCH --input - <<'EOF'
{
  "security_and_analysis": {
    "secret_scanning": {"status": "enabled"},
    "secret_scanning_push_protection": {"status": "enabled"}
  }
}
EOF
```

## Topics

```bash
gh api repos/fox-in-the-box-ai/fox-fleet/topics -X PUT --input - <<'EOF'
{"names": ["fleet-management", "docker", "self-hosted", "ai-assistant", "apache-2"]}
EOF
```
