#!/usr/bin/env bash
set -euo pipefail

blue()  { echo -e "\033[0;34m$*\033[0m"; }
green() { echo -e "\033[0;32m$*\033[0m"; }
yellow(){ echo -e "\033[1;33m$*\033[0m"; }
red()   { echo -e "\033[0;31m$*\033[0m"; }

header() { echo -e "\n\033[1;34m=== $* ===\033[0m\n"; }

PROJECT_ROOT_MARKERS=(README.md docker-compose.yml backend frontend)

ensure_project_root() {
  local found=0
  for m in "${PROJECT_ROOT_MARKERS[@]}"; do
    [[ -e "$m" ]] && found=$((found+1))
  done
  if [[ $found -lt 2 ]]; then
    red "Run this from the repository root (where README.md and backend/ exist)."
    exit 1
  fi
}

check_gitignore_env() {
  header "Checking .gitignore protections"
  if rg -n "^\\.env(\\..*)?$" .gitignore >/dev/null 2>&1; then
    green "✅ .gitignore ignores .env files"
  else
    yellow "⚠️ .gitignore does not explicitly ignore .env files"
    yellow "   Add the following lines to .gitignore to avoid committing secrets:"
    echo -e "\n# Environment variables\n.env\n.env.local\n.env.*.local\n"
  fi
}

create_backend_env_if_missing() {
  header "Bootstrapping backend/.env (local only)"
  if [[ -f backend/.env ]]; then
    green "✅ backend/.env already exists (not touching)"
  else
    if [[ -f backend/.env.example ]]; then
      cp backend/.env.example backend/.env
      chmod 600 backend/.env || true
      green "✅ Created backend/.env from template (placeholders only)"
      yellow "ℹ️  Populate backend/.env for local dev; production uses GitHub Secrets"
    else
      red "backend/.env.example is missing. Cannot create backend/.env"
      exit 1
    fi
  fi
}

init_detect_secrets_baseline() {
  header "Optional: secrets baseline"
  if command -v detect-secrets >/dev/null 2>&1; then
    if [[ -f .secrets.baseline ]]; then
      green "✅ .secrets.baseline already exists"
    else
      yellow "Generating .secrets.baseline (no secrets output will be shown)"
      detect-secrets scan --baseline .secrets.baseline || true
      green "✅ Baseline created (.secrets.baseline)"
    fi
  else
    yellow "⚠️ detect-secrets not installed. Skipping baseline generation."
    echo "Install: pipx install detect-secrets  (or) pip3 install --user detect-secrets"
  fi
}

print_next_steps() {
  header "Next steps"
  echo "1) Configure GitHub Secrets for production deploy (recommended):"
  echo "   - See SECRETS_SETUP.md for the complete list and flow"
  echo "2) For browser-based large uploads, set these in backend/.env (or GitHub Secrets):"
  echo "   - MINIO_PUBLIC_ENDPOINT=<public host or IP>:9000"
  echo "   - MINIO_PUBLIC_SECURE=false   # or true if using HTTPS"
  echo "3) Validate CI/CD and Pi connectivity before pushing:"
  echo "   - ./pre-deploy-check.sh"
  echo "4) Run with placeholders only; never commit real secrets."
}

main() {
  header "Secure Init – Sermon Uploader"
  ensure_project_root
  check_gitignore_env()
  create_backend_env_if_missing()
  init_detect_secrets_baseline()
  print_next_steps
  echo
  green "✅ Secure init complete"
}

main "$@"

