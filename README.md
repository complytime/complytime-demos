# ComplyTime Demos

This repository provides a demo and testing environment for
[complyctl](https://github.com/complytime/complyctl) and its
[providers](https://github.com/complytime/complytime-providers).

It uses Vagrant to spin up Fedora VMs and Ansible playbooks to build,
install, and configure complyctl with its providers for compliance
scanning tests.

## Architecture

The environment uses two simple, widely available tools:

- **Vagrant**: Spins up a Fedora 44 VM with libvirt, installs essential
  packages, and creates an Ansible user.
- **Ansible**: Configures the VM in a reproducible way via playbooks.

Setup playbooks prepare the VM with tools and content:

| Playbook | Purpose |
|----------|---------|
| `populate_complyctl_dev_binaries.yml` | Build binaries locally, copy to VM. Fast iteration for development. |
| `populate_complyctl_dev_rpm.yml` | Build RPMs from local branches on the VM. Simulates Fedora packaging. |
| `populate_complyctl_dev_content.yml` | Configure policies and fetch content for compliance scanning. |

Demo playbooks run end-to-end assessment scenarios:

| Playbook | Purpose | Guide |
|----------|---------|-------|
| `demo_complyctl_fedora.yml` | CIS benchmark scan on the Fedora VM using the OpenSCAP provider. | |
| `demo_complyctl_github.yml` | Branch protection scan on GitHub using the Ampel provider. | |
| `demo_complyctl_merge_readiness.yml` | Merge readiness gate combining CI health and branch protection. | [docs/demo-merge-readiness.md](docs/demo-merge-readiness.md) |

All playbooks require local clones of two repositories:

- [complytime/complyctl](https://github.com/complytime/complyctl)
- [complytime/complytime-providers](https://github.com/complytime/complytime-providers)

## Directory Structure

```
complytime-demos/
├── base_ansible_env/               # Ansible configuration, inventory, playbooks
│   ├── templates/                  # Jinja2 templates (complytime.yaml.j2)
│   ├── ansible_inventory           # Auto-populated by populate_ansible_inventory.sh
│   └── ansible.cfg                 # Ansible configuration for this directory
├── base_vms/                       # VM definitions
│   ├── fedora/                     # Fedora VM (Vagrantfile)
│   ├── rhel9/                      # RHEL 9 VM (Vagrantfile)
│   └── populate_ansible_inventory.sh
└── README.md
```

## Prerequisites

- [Vagrant](https://www.vagrantup.com/) with the
  [vagrant-libvirt](https://github.com/vagrant-libvirt/vagrant-libvirt)
  plugin
- [Ansible](https://docs.ansible.com/ansible/latest/installation_guide/)
- Local clones of
  [complyctl](https://github.com/complytime/complyctl) and
  [complytime-providers](https://github.com/complytime/complytime-providers)
- Internet access (for `vagrant up`, `complyctl get`, and
  `go_vendor_archive`)

## Getting Started

### Step 1: Create the Fedora VM

```bash
cd base_vms/fedora
vagrant up
```

The Vagrantfile uses `qemu:///session` (user-level libvirt). The VM
image is downloaded from the Fedora mirrors on first run.

It is recommended to create a snapshot of the fresh VM before running
playbooks. This saves time when experimenting with different
configurations:

```bash
vagrant snapshot save fresh
# To restore later: vagrant snapshot restore fresh
```

### Step 2: Install complyctl and providers

From the `base_ansible_env/` directory, choose one of the two
installation workflows below.

**Default repository paths** are defined in each playbook's `vars`
section. Override them if your local clones are in different locations
using `-e`:

```bash
cd base_ansible_env
```

#### Option A: Binary workflow (fast development testing)

Builds complyctl and all providers locally on the host, then copies
binaries to the VM. Providers are installed to
`~/.local/share/complytime/providers/` (complyctl's XDG discovery
path). Also installs runtime dependencies: `snappy`, `ampel` (via
`go install`), and `conftest` (pre-built binary).

```bash
ansible-playbook populate_complyctl_dev_binaries.yml

# Or with custom repository paths:
ansible-playbook populate_complyctl_dev_binaries.yml \
  -e complyctl_repo_dest=~/path/to/complyctl \
  -e providers_repo_dest=~/path/to/complytime-providers
```

After completion, verify on the VM:

```bash
vagrant ssh  # from base_vms/fedora/
~/bin/complyctl version
ls ~/.local/share/complytime/providers/
```

#### Option B: RPM workflow (Fedora packaging simulation)

Builds RPMs from your local working branches on the VM using
`go_vendor_archive` and `rpmbuild`, then installs them via `dnf`.
This simulates how the packages behave when released in Fedora.

The RPM version is derived from the latest release git tag reachable
from HEAD (pre-release tags like `v1.0.0-rc.0` are excluded since RPM
does not allow hyphens in the Version field).

**Note**: `git archive` only captures committed content. Uncommitted
changes will not be included in the RPM build.

```bash
ansible-playbook populate_complyctl_dev_rpm.yml

# Or with custom repository paths:
ansible-playbook populate_complyctl_dev_rpm.yml \
  -e complyctl_repo_dest=~/path/to/complyctl \
  -e providers_repo_dest=~/path/to/complytime-providers
```

After completion, verify on the VM:

```bash
vagrant ssh  # from base_vms/fedora/
complyctl version
rpm -q complyctl complytime-providers-openscap \
  complytime-providers-ampel complytime-providers-opa
```

### Step 3: Populate compliance content

After installing complyctl and providers (via either workflow), run
the content playbook to configure policies and fetch content:

```bash
ansible-playbook populate_complyctl_dev_content.yml
```

This playbook:

- Creates a `complytime.yaml` configuration pointing to the ampel
  branch-protection policy on quay.io
- Downloads ampel granular policy files from the
  [org-infra](https://github.com/complytime/org-infra) repository
  (canonical source of truth)
- Runs `complyctl get` to fetch policies
- Validates the environment with `complyctl doctor`

To use a local clone of org-infra instead of downloading from GitHub:

```bash
ansible-playbook populate_complyctl_dev_content.yml \
  -e org_infra_local_path=~/path/to/org-infra
```

### Step 4: Test compliance scanning

Connect to the VM and run the compliance workflow manually:

```bash
cd ~/complyctl-demo

# Generate the ampel policy bundle from granular policies
complyctl generate --policy-id ampel-bp

# Scan the target repository against the policy
complyctl scan --policy-id ampel-bp
```

## Playbook Variables Reference

### populate_complyctl_dev_binaries.yml

| Variable | Default | Description |
|----------|---------|-------------|
| `complyctl_repo_dest` | `~/GIT/ProdSec/complyctl` | Path to local complyctl clone |
| `providers_repo_dest` | `~/GIT/ProdSec/complytime-providers` | Path to local providers clone |
| `snappy_version` | `v0.2.6` | Pinned snappy tool version |
| `ampel_version` | `v1.3.6` | Pinned ampel tool version |
| `conftest_version` | `0.69.0` | Pinned conftest version |

### populate_complyctl_dev_rpm.yml

| Variable | Default | Description |
|----------|---------|-------------|
| `complyctl_repo_dest` | `~/GIT/ProdSec/complyctl` | Path to local complyctl clone |
| `providers_repo_dest` | `~/GIT/ProdSec/complytime-providers` | Path to local providers clone |
| `snappy_version` | `v0.2.6` | Pinned snappy tool version |
| `ampel_version` | `v1.3.6` | Pinned ampel tool version |
| `conftest_version` | `0.69.0` | Pinned conftest version |

### populate_complyctl_dev_content.yml

| Variable | Default | Description |
|----------|---------|-------------|
| `complyctl_workdir` | `~/complyctl-demo` | Working directory on the VM |
| `org_infra_ref` | `main` | Branch/tag of org-infra to pull policies from |
| `org_infra_local_path` | *(commented out)* | Local org-infra clone path (overrides download) |

### demo_complyctl_fedora.yml

| Variable | Default | Description |
|----------|---------|-------------|
| `complyctl_workdir` | `~/complyctl-demo` | Working directory on the VM |
| `policy_id` | `cis-fedora-l1-server` | Policy ID for CIS Fedora L1 Server |

### demo_complyctl_github.yml

| Variable | Default | Description |
|----------|---------|-------------|
| `complyctl_workdir` | `~/complyctl-demo-ampel` | Working directory on the VM |
| `snappy_version` | `v0.2.6` | Pinned snappy tool version |
| `ampel_version` | `v1.3.6` | Pinned ampel tool version |

## Automated Demos

As the [complytime projects](https://github.com/complytime/) evolve,
more complete automated demos are being developed. The playbooks
themselves serve as reference for manual exploration.

### demo_complyctl_fedora.yml

Self-contained OpenSCAP CIS L1 Server compliance demo. This playbook
installs its own dependencies — no `populate_*` playbooks are needed.

**Prerequisites**: `vagrant up` only (see [Step 1](#step-1-create-the-fedora-vm)).

**Usage**:

```bash
cd base_ansible_env/
ansible-playbook demo_complyctl_fedora.yml
```

**What it does**:

1. Installs `complyctl` and the OpenSCAP provider via `dnf`
2. Creates a workspace with an inline `complytime.yaml` configuration
3. Fetches the CIS Fedora L1 Server policy from `quay.io`
4. Deliberately breaks three rules to create non-compliance:
   - Removes `firewalld` (`package_firewalld_installed`)
   - Removes sudo logfile configuration (`sudo_custom_logfile`)
   - Sets weak UMASK (`accounts_umask_etc_login_defs`)
5. Scans the system and captures results showing failures
6. Fixes all three rules and re-scans to show improvement
7. Displays a before/after comparison of scan results

Results are fetched to `./downloads_complyctl_fedora_scan1/` (before fixes) and
`./downloads_complyctl_fedora_scan2/` (after fixes) on the host machine.

### demo_complyctl_github.yml

Self-contained Ampel branch-protection compliance demo. This playbook
installs its own dependencies — no `populate_*` playbooks are needed.

**Prerequisites**: `vagrant up` (see [Step 1](#step-1-create-the-fedora-vm))
and a `GITHUB_TOKEN` environment variable.

**Token setup**:

```bash
export GITHUB_TOKEN=$(gh auth token)
```

The token requires `administration:read` scope for branch protection
rule queries.

**Usage**:

```bash
cd base_ansible_env/
ansible-playbook demo_complyctl_github.yml
```

**What it does**:

1. Validates that `GITHUB_TOKEN` is set (fails with guidance if missing)
2. Installs `complyctl` and the Ampel provider via `dnf`
3. Installs `snappy` and `ampel` CLI tools via `go install`
4. Creates a workspace with an inline `complytime.yaml` configuration
   targeting the `complytime-demos` GitHub repository
5. Fetches the `ampel-bp` policy from `quay.io`
6. Scans branch protection rules and fetches results

The token is handled securely via `no_log: true` — safe for recorded
sessions. Results are fetched to `./downloads_complyctl_github/` on the host
machine.

**Note**: Go tool versions are pinned by default. Override with
`-e snappy_version=latest -e ampel_version=latest`.

