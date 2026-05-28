FROM quay.io/centos/centos:stream9

RUN dnf upgrade -y && dnf install -y \
    qemu-guest-agent \
    openssh-clients \
    openssh-server \
    git \
    python3-pip \
    tar \
    tree \
    vim
RUN dnf install -y --nobest openscap-scanner openscap-engine-sce scap-security-guide
