# ComplyTime Demos

This repository is intended to be used as base for [complyctl](https://github.com/complytime/complyctl) and [complyscribe](https://github.com/complytime/complyscribe) Demos.

As complyctl and complyscribe are evolving on their features, as well as [CaC/content](https://github.com/complianceAsCode/content) are being transformed to OSCAL and vice versa, we can show more complex demos with real content.

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
├── base_vms/                       # Centralize instructions to create demo VMs
│ ├── fedora                        # Instructions to create a fedora demo VM
│ │   └── Vagrantfile               # Vagrant instructions to create a local fedora VM
│ ├── rhel9                         # Instructions to create a rhel9 demo VM
│ │   └── Vagrantfile               # Vagrant instructions to create a local rhel9 VM
│ └── populate_ansible_inventory.sh # Script to collect information from Vagrant VM and populate the Ansible inventory
├── scripts/                        # Supporting scripts (WIP)
├── CONTENT_TRANSFORMATION.md       # Examples of commands used in trestle-bot to generate OSCAL content based in ComplianceAsCode/content
└── README.md                       # Main file to centralize instructions and other relevant information for demos
```

## Get Your Hands Dirty

### Base RHEL 9 VM

```bash
git clone https://github.com/complytime/complytime-demos.git
cd complytime-demos/base_vms/rhel9
vagrant up
```

It is recommended to create a snapshot of the fresh VM if you plan to work on a new Demo or experiment different Demos.
This way you can save time provisioning a new Vagrant Box.

### Base Fedora VM

```bash
cd complytime-demos/base_vms/fedora
vagrant up
```

#### Connect to the Demo VM

You can connect using vagrant command:
```bash
vagrant ssh
```

Or you can connect via SSH using the hint from `populate_ansible_inventory.sh` script. e.g.:
```bash
ssh ansible@192.168.122.161
```

### Populate complyctl binaries

Execute the `populate_complyctl_dev_binaries.yml` Playbook to build complyctl binaries locally and send them to the Demo VM.
For now, the complyctl binaries are built locally, so it is required the `https://github.com/complytime/complyctl.git` repository cloned and the minimal packages necessary to build Go code. More information could be found [here](https://github.com/complytime/complytime/blob/main/docs/INSTALLATION.md)

Once complyctl can be built locally, there is a green light to move forward with the Ansible Playbooks.

```bash
cd ../base_ansible_env
# Make sure the "complyctl_repo_dest" variable in this Playbook is aligned to the directory where the complyctl repository was previously cloned.
ansible-playbook populate_complyctl_dev_binaries.yml
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

In order to speed up the tests, OSCAL content transformed from CaC can be obtained from https://github.com/ComplianceAsCode/oscal-content.
The default content set in variables is based on anssi_bp28_minimal profile for RHEL 9.
Feel free to select your preferred content for tests by updating the Playbook variables with the respective URL. For example, to use the `fedora-cusp` content from the `ComplianceAsCode/oscal-content` repository, you can use the following variables:

```yaml
   catalog: "https://raw.githubusercontent.com/ComplianceAsCode/oscal-content/refs/heads/main/catalogs/cusp_fedora/catalog.json"
   profile: "https://raw.githubusercontent.com/ComplianceAsCode/oscal-content/refs/heads/main/profiles/fedora-cusp_fedora-default/profile.json"
   component_definition: "https://raw.githubusercontent.com/ComplianceAsCode/oscal-content/refs/heads/main/component-definitions/fedora/fedora-cusp_fedora-default/component-definition.json"
```

```bash
ansible-playbook populate_complyctl_dev_content.yml
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
│           │   └── test-component-definition.json
│           ├── controls
│           │   └── test-profile.json
│           │   └── test-catalog.json
│           └── plugins
│               ├── c2p-openscap-manifest.json
│               └── openscap-plugin
...
```

### Generating OSCAL Content from CaC/content transformation

For reference, the commands for transforming CaC/content are organized by policy_id in the [CONTENT_TRANSFORMATION.md](https://github.com/complytime/complytime-demos/blob/d403cb455f4bf6f4e4dd9e7d7fc724d9e0b0e321/CONTENT_TRANSFORMATION.md).

### Try complyctl commands

Once the Demo VM is populated with ComplyTime binaries and OSCAL content, here are some nice commands to try:
```bash
complyctl list
complyctl info anssi_bp28_minimal
complyctl plan anssi_bp28_minimal
complyctl generate
complyctl scan
tree -a
```

## Automated Demos

As the [complytime projects](https://github.com/complytime/) evolve and more features are included, more complete demos can be showed.
But these demos can easily get long if executed manually. For this reason, this repository also provides Playbooks with automated demos.

The Playbooks themselves serve as reference to manually explore in more details any specific step on-demand.
With this context, the recommended way to consume these Playbooks is executing them and analyzing the outputs.
Another recommendation is to connect to the Demo VM and trying to reproduce some commands from the Playbook.

### demo_complyctl_fedora.yml

#### Requirements

- [Vagrant Box with Fedora 42](#base-fedora-vm)

```bash
ansible-playbook demo_complyctl_fedora.yml
```

### run_complybeacon_fedora.yml

This playbook will copy your local complybeacon dir to a fedora vm, then configure all things needed to run the whole complybeacon stack. After running this playbook, you will get a complybeacon environment ready to use.

#### Requirements

- [Vagrant Box with Fedora 42](#base-fedora-vm)

`complybeacon_local_dir` is your local complybeacon dir

```bash
cd base_ansible_env/
ansible-playbook run_complybeacon_fedora.yml -e "complybeacon_local_dir=/develop/fork/complybeacon"
```
