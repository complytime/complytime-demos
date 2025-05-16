# ComplyTime Demos

This repository is intended to be used as base for [ComplyTime](https://github.com/complytime/complytime) and [trestlebot](https://github.com/complytime/trestle-bot) Demos.

As ComplyTime and trestlebot are evolving on their features, as well as [CaC/content](https://github.com/complianceAsCode/content) are being transformed to OSCAL and vice-versa, we can show more complex demos with real content.

This repository targets some goals:
* Standardize the demos so they can be easily extended
* Provide a consistent experience along the demos
* Allow the team and stakeholders to reproduce the demos on their computers

## Architecture

The idea is pretty simple: Use simple and easily available tools so a wider audience can quickly on-board.

These tools are:
* Vagrant: Used to spin up a VM with custom repositories, some essential packages and an Ansible user
* Ansible: Used to configure the VM in an easily reproducible way for each demo

## Directory Structure

```
complytime-demos/
├── base_ansible_env/               # Centralize Ansible configuration, inventory, Playbooks and the resources used by Playbooks
│ ├── files/                        # Sample files used by Playbooks
│ ├── templates/                    # Jinja2 templates used by Playbooks
│ ├── ansible_inventory             # This file is automatically updated by "populate_ansible_inventory.sh"
│ └── ansible.cfg                   # Ansible configuration file specific for "base_ansible_env" directory
├── base_rhel9_env/                 # Centralize instructions to create a rhel9 demo VM
│ ├── populate_ansible_inventory.sh # Script to collect information for Vagrant VM and populate the Ansible inventory
│ └── Vagrantfile                   # Vagrant instructions to create a local VM
├── scripts/                        # Supporting scripts (WIP)
└── README.md                       # Main file to centralize instructions and other relevant information for demos.s
```

## Get Your Hands Dirty

### Base RHEL 9 VM

```bash
git clone https://github.com/marcusburghardt/complytime-demos.git
cd complytime-demos/base_rhel9_vm
vagrant up
./populate_ansible_inventory.sh
```

It is recommended to create a snapshot of the fresh VM if you plan to work on a new Demo or experiment different Demos.
This way you can save time provisioning a new Vagrant Box.

#### Connect to the Demo VM

You can connect using vagrant command:
```bash
vagrant ssh
```

Or you can connect via SSH using the hint from `./populate_ansible_inventory.sh` script. e.g.:
```bash
ssh ansible@192.168.122.161
```

### Populate ComplyTime binaries

Execute the `populate_complytime_vm.yml` Playbook to build ComplyTime binaries locally and send them to the Demo VM.
For now, the ComplyTime binaries are built locally, so it is required the `https://github.com/complytime/complytime.git` repository cloned and the minimal packages necessary to build Go code. More information could be found [here](https://github.com/complytime/complytime/blob/main/docs/INSTALLATION.md)

Once ComplyTime can be built locally, there is a green light to move forward with the Ansible Playbooks.

```bash
cd ../base_ansible_env
# Make sure the "complytime_repo_dest" variable in this Playbook is aligned to the directory where the ComplyTime repository was previously cloned.
ansible-playbook populate_complytime_vm.yml
```

After running this Playbook a directory structure similar to this is expected in /home/ansible:
```
...
├── bin
│   └── complytime
├── .local
│   └── share
│       └── complytime
│           ├── bundles
│           ├── controls
│           └── plugins
│               ├── c2p-openscap-manifest.json
│               └── openscap-plugin
...
```

### Populate OSCAL Content

In order to speed up the tests, this repository contains OSCAL content transformed from CaC based on ansi_bp28_minimal profile for RHEL 9.

```bash
ansible-playbook populate_complytime_anssi_content.yml
```

After running this Playbook a directory structure similar to this is expected in /home/ansible:
```
...
├── bin
│   └── complytime
├── .local
│   └── share
│       └── complytime
│           ├── bundles
│           │   └── anssi-component-definition.json
│           ├── controls
│           │   └── anssi-minimal-profile.json
│           │   └── anssi-catalog.json
│           └── plugins
│               ├── c2p-openscap-manifest.json
│               └── openscap-plugin
...
```

#### Regenerate ANSSI content
For reference, these were the commands used with trestlebot to transform the ANSSI content in this example:

```bash
# Create a OSCAL catalog based on ANSSI control file in CaC/content
poetry run trestlebot catalog --cac-content-root ~/CaC/Forks/content --cac-policy-id anssi --repo-path ~/LABs/trestlebot-labs --oscal-catalog anssi --branch main --committer-name test --committer-email test@redhat.com --dry-run

# Once a catalog is available, the ANSSI profiles can be created based on control file information in CaC/content
poetry run trestlebot sync-cac-content profile --product rhel9 --cac-content-root ~/CaC/Forks/content --cac-policy-id anssi --repo-path ~/LABs/trestlebot-labs/ --oscal-catalog anssi --committer-name test --committer-email test@redhat.com --branch main --dry-run

# With a profile available, an OSCAL Component Definition can be created using information from anssi_bp28_minimal profile for RHEL 9 product
poetry run trestlebot sync-cac-content component-definition --product rhel9 --cac-content-root ~/CaC/Forks/content --cac-profile anssi_bp28_minimal --repo-path ~/LABs/trestlebot-labs --oscal-profile anssi-minimal --component-definition-type software --committer-name test --committer-email test@redhat.com --branch main --dry-run

# Finally the new Component Definition can be updated to include a validation component, to be used by openscap-plugin later
poetry run trestlebot sync-cac-content component-definition --product rhel9 --cac-content-root ~/CaC/Forks/content --cac-profile ~/CaC/Forks/content/products/rhel9/profiles/anssi_bp28_minimal.profile --repo-path ~/LABs/trestlebot-labs --oscal-profile anssi-minimal --component-definition-type validation --committer-name test --committer-email test@redhat.com --branch main --dry-run
```

After these commands, the generated component definition and the chosen profile files were copied to `base_ansible_env/files` to be used with `populate_complytime_anssi_content.yml` Playbook.

### Generating OSCAL Content from CaC/content transformation

The commands for transforming CaC/content are organized by policy_id in the [CONTENT_TRANSFORMATION.md](https://github.com/complytime/complytime-demos/blob/d403cb455f4bf6f4e4dd9e7d7fc724d9e0b0e321/CONTENT_TRANSFORMATION.md).


### Try ComplyTime commands

Once the Demo VM is populated with ComplyTime binaries and OSCAL content, here are some nice commands to try:
```bash
complytime list
complytime plan anssi_bp28_minimal
complytime generate
complytime scan
tree -a
```
