# 🔒 Recipe: Secure Init (No Secrets in Git)

> Goal: Bootstrap safely without exposing secrets  
> Time: ⏱️ 3–5 minutes  
> Difficulty: 🟢 Easy

## 📦 What You Need
- [ ] Cloned repo locally
- [ ] Bash shell
- [ ] Optional: `detect-secrets` installed

## 🎯 End Result
- Local `backend/.env` created from a safe template (placeholders only)
- `.env` files ignored by git
- Optional `.secrets.baseline` generated for scanning
- Clear next steps to use GitHub Secrets in CI/CD

## 📝 Steps

### Step 1: Run Secure Init (1 min)
```bash
bash scripts/secure-init.sh
```
- Verifies `.gitignore` protections
- Creates `backend/.env` if missing (from `backend/.env.example`)
- Optionally generates `.secrets.baseline` (if `detect-secrets` installed)
- Prints next steps for secrets handling

### Step 2: Configure Local Dev (2–3 min)
Edit `backend/.env` for local-only use (never commit):
```bash
# Minimal local example
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=local-only
MINIO_SECRET_KEY=local-only
MINIO_BUCKET=sermons
PORT=8000
```
- For browser-based large uploads via presigned URLs, set:
```bash
MINIO_PUBLIC_ENDPOINT=192.168.1.127:9000   # or minio.example.com
MINIO_PUBLIC_SECURE=false                   # true if using HTTPS
```

### Step 3: Use GitHub Secrets for Production (2 min)
- Follow SECRETS_SETUP.md to add repository secrets (no secrets in code)
- CI/CD injects env vars during deploy; Pi receives `.env` server-side

## ✅ Success Check
- ✓ `backend/.env` exists locally and is gitignored
- ✓ `git status` shows no `.env` files
- ✓ (Optional) `.secrets.baseline` present in repo root

## 🚨 If Something’s Wrong
- “.env.example missing”: ensure `backend/.env.example` exists in repo (it does)  
- “detect-secrets not found”: install with `pipx install detect-secrets` or `pip3 install --user detect-secrets`  
- “Uploads failing for large files”: verify `MINIO_PUBLIC_ENDPOINT` is set to a client-reachable host

## 💡 Pro Tips
- Never print secrets in console logs or commit messages
- Keep `backend/.env` for local only; use GitHub Secrets for deploys
- Run `./pre-deploy-check.sh` before pushing to save CI minutes

## 🔗 Related
- SECRETS_SETUP.md  
- DEPLOYMENT.md  
- APIs/Presigned Upload APIs

