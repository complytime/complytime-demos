packer {
  required_plugins {
    docker = {
      version = ">= 1.0.8"
      source  = "github.com/hashicorp/docker"
    }
  }
}

source "docker" "ubuntu" {
  image  = "quay.io/centos/centos:stream9"
  commit = true
}

build {
  name = "complytime"
  sources = [
    "source.docker.ubuntu"
  ]

  provisioner "shell" {
    inline = [
      # Install required packages
      "dnf install -y qemu-guest-agent openssh-clients openssh-server",

      # Install some useful packages
      "dnf install -y git python3-pip tar tree vim -y",
      "dnf install -y --nobest openscap-scanner openscap-engine-sce scap-security-guide",

      # Upgrade the system
      "dnf upgrade -y",
      ]
  }
}

