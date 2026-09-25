# Deployment runbook

Pushes to `main` on GitHub deploy Hooklook to the prepared VPS through the
[`Deploy production`](../.github/workflows/deploy.yml) workflow. The VPS pulls
a published container image from GHCR; it never clones the repository and
never builds Hooklook.

Hooklook does not ship its own public ingress. On the production VPS, TLS, the
`hooklook.app` route, the public rate, body, and header limits, and the
`hooklook-edge` Docker network all belong to
[hetzner-one](https://github.com/dvinubius/hetzner-one), a separate Compose
project deployed at `/opt/caddy`. This runbook assumes hetzner-one is already
running. Without it, a full deployment stops at the `hooklook-edge` check, and
the site is not publicly reachable. This runbook does not provision a VM,
install Docker, configure DNS, or change Caddy.

## How a push deploys

1. **Test.** Frontend tests and build, `go vet`, Go tests, the deployment,
   backup, and verifier shell tests, dashboard JSON validation, and Compose
   rendering. Nothing contacts the VPS before these pass.
2. **Plan.** Runs serialized in the `production-deploy` concurrency group. It
   reads `/opt/hooklook/.deploy/manifest` over SSH and passes the last
   verified commit to
   [`classify-deploy.sh`](../scripts/classify-deploy.sh), which inspects every
   path changed since then:

   | Changed since the last verified deployment | Mode |
   | --- | --- |
   | Only docs, agent notes, tests, `Makefile`, `.env.example` | none |
   | `observability/grafana/dashboards/hooklook.json` (plus docs) | dashboard |
   | Other `observability/` files (plus docs) | observability |
   | Dashboard and other observability files together | full |
   | `compose.yaml`, app, frontend, Dockerfile, scripts, workflow, or any unlisted path | full |
   | No manifest, or its commit is not an ancestor | full |

   If SSH or Git inspection fails, the run stops without changing production.
   A docs-only push leaves the manifest unchanged, so those paths are simply
   seen again next time.
3. **Image** (full mode only). Builds the Dockerfile for `linux/amd64`, pushes
   `ghcr.io/dvinubius/hooklook:<commit>`, and passes on the immutable
   `ghcr.io/dvinubius/hooklook@sha256:…` reference. Only this job can write
   packages.
4. **Deploy.** Confirms the commit is still the head of `main` (otherwise the
   run is stale and a newer run will deploy), creates a bundle with
   `git archive` from that exact commit (`compose.yaml`, `observability/`,
   and the operational `scripts/`, without their tests), uploads it to
   `/opt/hooklook/.deploy/staging/<commit>`, and runs its
   [`remote-deploy.sh`](../scripts/remote-deploy.sh).

The remote script holds a host lock, runs its preconditions, saves a snapshot
of the live `compose.yaml`, `observability/`, `scripts/`, `.env.image`, and
manifest, and then applies one mode:

- **full**: checks Docker, `curl`, and the `hooklook-edge` network; runs the
  [storage headroom check](storage-capacity.md); pulls the exact digest (with
  the run's short-lived `GITHUB_TOKEN`, deleted after the pull); installs the
  bundle; atomically writes `.env.image`; recreates Hooklook with
  `--no-build`; checks loopback `/health` and `/ready`, the running
  container, and public `/health`; recreates the telemetry services; and runs
  the full telemetry smoke test.
- **observability**: requires a running Hooklook and a GHCR image already
  deployed; replaces only `observability/`, recreates Prometheus, Alloy,
  Loki, and Grafana, and runs the full telemetry smoke test. It does not pull
  or recreate the app.
- **dashboard**: requires a running Grafana; replaces only the dashboard JSON
  and runs the dashboard smoke test. It runs no Compose `up`.

Only after every check passes does it write the mode-0600 manifest
(`commit`, `image`, `mode`, `deployed_at`). On a failed check it prints
bounded diagnostics, restores the snapshot, and verifies the restored
deployment; a rollback that also fails is reported as `ROLLBACK FAILED`. Five
snapshots are kept in `/opt/hooklook/.deploy/snapshots`.

Deployment never touches `.env`, `.env.observability`, the named volumes,
Caddy, or zibs (another application behind the same Caddy), and never runs `docker compose down`.

## One-time setup

### Shared ingress

Deploy hetzner-one first by following its
[deployment runbook](https://github.com/dvinubius/hetzner-one/blob/main/docs/deployment-runbook.md).
It creates `hooklook-edge`, obtains the `hooklook.app` certificate, and
proxies that hostname to `hooklook:8080`. DNS for `hooklook.app` must already
point at the VPS. Until Hooklook joins the network, Caddy answers
`hooklook.app` with `502`; that is expected before the first deployment.

### GitHub repository

The workflow expects a GitHub environment named `production` holding:

| Name | Kind | Value |
| --- | --- | --- |
| `DEPLOY_SSH_KEY` | secret | Private key of the dedicated deployment key pair |
| `DEPLOY_HOST` | variable | VPS address |
| `DEPLOY_USER` | variable | `hooklook-deploy` |
| `DEPLOY_KNOWN_HOSTS` | variable | Verified `known_hosts` line(s) for `DEPLOY_HOST` |

Environment secrets and branch protection on a private repository require a
paid GitHub plan; on GitHub Free, store the same names as repository secrets
and variables instead. The repository is public, so both are available.

Create the environment first (`gh secret set --env` fails when it does not
exist) and allow deployments from `main` only:

```bash
gh api -X PUT repos/dvinubius/hooklook/environments/production \
  -F 'deployment_branch_policy[protected_branches]=false' \
  -F 'deployment_branch_policy[custom_branch_policies]=true'
gh api -X POST repos/dvinubius/hooklook/environments/production/deployment-branch-policies \
  -f name=main -f type=branch
```

Generate the dedicated key locally and never reuse the operator's key:

```bash
ssh-keygen -t ed25519 -N '' -C hooklook-github-deploy -f ~/.ssh/hooklook_github_deploy
```

Take the host key from a channel you already trust, such as an existing SSH
session, and never from an unverified `ssh-keyscan` in the workflow. On the
VPS, print the line with the address written exactly as `DEPLOY_HOST` will
hold it (an IP and a hostname do not match each other):

```bash
printf '%s %s\n' '<vps address>' "$(cut -d' ' -f1,2 /etc/ssh/ssh_host_ed25519_key.pub)"
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

Its fingerprint must match the one your workstation already trusts
(`ssh-keygen -F '<vps address>' -l`).

Then, from the repository:

```bash
gh secret set DEPLOY_SSH_KEY --env production <~/.ssh/hooklook_github_deploy
gh variable set DEPLOY_HOST --env production --body "<vps address>"
gh variable set DEPLOY_USER --env production --body hooklook-deploy
gh variable set DEPLOY_KNOWN_HOSTS --env production --body "<known_hosts line>"
```

Protect `main` against force pushes and deletion, and require the `test` job.
A rewritten `main` leaves the manifest's commit off the branch's history,
which the classifier treats as a full deployment. Direct pushes by the
repository admin bypass the required check (GitHub reports the bypass); the
workflow still deploys nothing unless `test` passes.

```bash
gh api -X PUT repos/dvinubius/hooklook/branches/main/protection --input - <<'JSON'
{"required_status_checks":{"strict":false,"contexts":["test"]},"enforce_admins":false,"required_pull_request_reviews":null,"restrictions":null,"allow_force_pushes":false,"allow_deletions":false}
JSON
```

GHCR publication uses the workflow's `GITHUB_TOKEN`. The package is linked to
the repository through its `org.opencontainers.image.source` label and is
public, so anyone can pull it. The VPS still pulls with the run's short-lived
`GITHUB_TOKEN` and needs no PAT or Git credential.

### VPS deployment account

As root on the VPS, create an account used only for deployment, give it
Docker access, and hand it `/opt/hooklook`. Docker group membership is
effectively host-level privilege, which is why this key must not be the
operator's general key.

```bash
useradd --create-home --shell /bin/bash hooklook-deploy
usermod -aG docker hooklook-deploy
install -d -m 700 -o hooklook-deploy -g hooklook-deploy ~hooklook-deploy/.ssh
install -m 600 -o hooklook-deploy -g hooklook-deploy /dev/null ~hooklook-deploy/.ssh/authorized_keys
# Append the contents of ~/.ssh/hooklook_github_deploy.pub:
printf 'restrict %s\n' '<public key>' >>~hooklook-deploy/.ssh/authorized_keys

chown -R hooklook-deploy:hooklook-deploy /opt/hooklook
chmod 600 /opt/hooklook/.env /opt/hooklook/.env.observability
```

Before handing the key to GitHub, confirm from the workstation that it logs
in and reaches Docker:

```bash
ssh -i ~/.ssh/hooklook_github_deploy -o IdentitiesOnly=yes hooklook-deploy@<vps address> 'id; docker ps --format "{{.Names}}"'
```

`flock` (util-linux), `curl`, Docker, and the Compose plugin must be
installed. The runtime secrets in `.env` and `.env.observability` stay owned
by the VPS; to change one, edit the file on the host and run
`./scripts/compose.sh up -d --no-build` (see below).

### SSH reachability and hardening

GitHub-hosted runners connect from changing addresses that cannot be
allowlisted, so the Hetzner Cloud firewall allows inbound TCP 22 from any
IPv4 and IPv6 source, and `ci-deploy.sh` assumes port 22. Before opening it,
make `sshd` accept keys only. Ubuntu leaves password login on unless it is
set, and the first value `sshd` reads wins, so use an early drop-in and keep
a root session open while testing:

```bash
printf '%s\n' 'PasswordAuthentication no' 'KbdInteractiveAuthentication no' \
  'PermitRootLogin prohibit-password' 'PubkeyAuthentication yes' \
  'MaxAuthTries 3' 'LoginGraceTime 20' 'MaxStartups 10:30:60' \
  >/etc/ssh/sshd_config.d/00-hardening.conf
sshd -t && systemctl reload ssh
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries|logingracetime|maxstartups) '
```

From a second terminal, key logins must still work and
`ssh -o PubkeyAuthentication=no root@<vps address>` must fail with
`Permission denied (publickey)`. If `sshd -T` lists `allowusers` or
`allowgroups`, add `hooklook-deploy` there.

fail2ban bans addresses that keep failing to log in, which trims bot noise;
the runners and operator keys do not fail, so they are not banned:

```bash
apt install fail2ban
printf '%s\n' '[sshd]' 'enabled = true' 'backend = systemd' 'maxretry = 5' \
  'findtime = 10m' 'bantime = 1h' 'bantime.increment = true' \
  >/etc/fail2ban/jail.d/sshd.local
systemctl enable --now fail2ban
fail2ban-client status sshd
```

An operator locked out by repeated failures unbans from the Hetzner web
console with `fail2ban-client set sshd unbanip <ip>`. Do not enable `ufw` for
rate limiting on this Docker host; Docker bypasses it for published ports and
a default-deny policy can cut off SSH or Caddy.

### First GHCR deployment

The existing host-built deployment has no manifest, so the first run is full.
Its snapshot tags the running host-built image as
`hooklook:rollback-<snapshot>` so a failed or later rollback can return to it.

1. Push `main`. The push itself triggers the workflow, which runs full mode
   because no manifest exists. Alternatively, run it manually:
   `gh workflow run deploy.yml --ref main`.
2. Confirm the run succeeded, then on the VPS check
   `cat /opt/hooklook/.deploy/manifest`, `./scripts/compose.sh ps`, the
   telemetry smoke test, Grafana's `127.0.0.1:3001` bind, and that Caddy and
   zibs are healthy.
3. Remove the source tree left by the old local deployment. Everything the
   GHCR deployment needs lives in `compose.yaml`, `observability/`,
   `scripts/`, the `.env*` files, and `.deploy/`:

   ```bash
   cd /opt/hooklook
   ls -A   # review first
   rm -rf -- *.go go.mod go.sum Dockerfile .dockerignore Makefile frontend docs \
     README.md AGENTS.md CLAUDE.md .devnotes.md .vscode .gitignore .env.example \
     webhook-inspector local-testing .codegraph .claude .DS_Store .previous-image
   ```

   Confirm no `.git` directory or Go/frontend source remains.

## Operating the deployed stack

Always use the wrapper, which loads the runtime environment and the pinned
`.env.image`:

```bash
cd /opt/hooklook
./scripts/compose.sh ps
./scripts/compose.sh logs --tail=100 hooklook
./scripts/compose.sh up -d --no-build   # e.g. after a reboot or a secret change
cat .env.image                          # the running image digest
cat .deploy/manifest                    # the last verified deployment
ls .deploy/snapshots
```

To inspect the effective storage limit without putting the token in shell
history:

```bash
cd /opt/hooklook
set -a
source .env
set +a
curl --fail -H "Authorization: Bearer $ADMIN_TOKEN" http://127.0.0.1:8081/admin/storage
```

If the loopback health check passes but the public check fails, inspect the
shared ingress project at `/opt/caddy` using hetzner-one's
[verify and diagnose](https://github.com/dvinubius/hetzner-one/blob/main/docs/deployment-runbook.md#verify-and-diagnose)
steps. Do not rerun a Hooklook deployment as an ingress repair.

## Roll back

A failed deployment restores its snapshot automatically. To reverse a
deployment that succeeded, restore an earlier snapshot on the VPS:

```bash
cd /opt/hooklook
ls .deploy/snapshots
./scripts/remote-deploy.sh rollback <snapshot-name>
```

A snapshot named `<time>-<mode>` holds the state from before that deployment.
Rollback takes a fresh snapshot, restores the files, `.env.image`, and the
manifest from the chosen one, recreates Hooklook and telemetry without
building, and runs the same checks. The restored manifest describes what now
runs, so the next push deploys everything since then. Afterwards, fix
forward with a push to `main` (a revert commit is fine); a manual
`gh workflow run deploy.yml --ref main` redeploys the current head of `main`
in full.

Old images stay in the local Docker image store and in GHCR, where nothing
deletes them automatically. Keep them at least as long as the snapshots that
reference them. Neither rollback changes the `hooklook-data` volume; never use
`docker compose down -v` or remove that volume during a code rollback.
