#!/bin/sh
set -e -o pipefail

# Usage:
# ./install.sh

VERSION=v0.2

# Function to get Linux distribution
get_distro() {
    DISTRO=$(egrep '^(ID)=' /etc/os-release| sed 's/"//g' | cut -f2 -d"=")
    if [[ $DISTRO != @(rhel|fedora|centos) ]]
    then
      echo "This Linux distro is not supported by the install script"
      exit 1
    fi

}

# Function to get system architecture
get_arch() {
    ARCH=$(uname -m)
}

# If RHEL, use subscription manager to register
register_subs() {
    sudo subscription-manager register --auto-attach
    sudo subscription-manager repos --enable=rhocp-4.7-for-rhel-8-x86_64-rpms
}

# Apply SElinux policies
apply_selinux_policy() {
    # sudo semanage fcontext -a -t container_runtime_exec_t /usr/local/bin/microshift ||
    #   sudo semanage fcontext -m -t container_runtime_exec_t /usr/local/bin/microshift
    # sudo mkdir -p /var/lib/kubelet/
    # sudo chcon -R -t container_file_t /var/lib/kubelet/
    # sudo chcon -R system_u:object_r:bin_t:s0 /usr/local/bin/microshift
    sudo setenforce 0
}

# Install dependencies
install_dependencies() {
    sudo dnf install -y \
    policycoreutils-python-utils \
    conntrack \
    iptables-services
}

# Install CRI-O depending on the distro
install_crio() {
    case $DISTRO in
      "fedora")
        sudo dnf module -y enable cri-o:1.20
        sudo dnf install -y cri-o cri-tools
      ;;
      "rhel")
        sudo dnf install cri-o cri-tools -y
      ;;
      "centos")
        CRIOVERSION=1.20
        OS=CentOS_8_Stream
        sudo curl -L -o /etc/yum.repos.d/devel:kubic:libcontainers:stable.repo https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/$OS/devel:kubic:libcontainers:stable.repo
        sudo curl -L -o /etc/yum.repos.d/devel:kubic:libcontainers:stable:cri-o:$CRIOVERSION.repo https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable:cri-o:$CRIOVERSION/$OS/devel:kubic:libcontainers:stable:cri-o:$CRIOVERSION.repo
        sudo dnf install -y cri-o cri-tools
      ;;
    esac
}


# CRI-O config to match Microshift networking values
crio_conf() {
    sudo sed -i 's/10.85.0.0\/16/10.42.0.0\/24/' /etc/cni/net.d/100-crio-bridge.conf

     if [ "$DISTRO" == "rhel" ]; then
        sudo sed -i 's|/usr/libexec/crio/conmon|/usr/bin/conmon|' /etc/crio/crio.conf 
     fi
}

# Start CRI-O
verify_crio() {
    sudo systemctl enable crio
    sudo systemctl restart crio

}

# Download and install kubectl
get_kubectl() {
    curl -LO "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl"
    sudo chmod +x ./kubectl
    sudo mv ./kubectl /usr/local/bin/kubectl

}

# Download and install microshift
get_microshift() {
    if [ $ARCH = "x86_64" ]; then
        curl -L https://github.com/redhat-et/microshift/releases/download/$VERSION/microshift-linux-amd64 -o microshift
        curl -L https://github.com/redhat-et/microshift/releases/download/$VERSION/release.sha256 -o release.sha256
    fi

    SHA=$(sha256sum microshift | awk '{print $1}')
    if [[ $SHA != $(cat release.sha256 | awk '{print $1}') ]]; then echo "SHA256 checksum failed" && exit 1; fi

    sudo chmod +x microshift
    sudo mv microshift /usr/local/bin/

    apply_selinux_policy

    cat << EOF | sudo tee /usr/lib/systemd/system/microshift.service
[Unit]
Description=Microshift

[Service]
WorkingDirectory=/usr/local/bin/
ExecStart=microshift run
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl enable microshift.service --now

}

# Locate kubeadmin configuration to default kubeconfig location
prepare_kubeconfig() {
    mkdir -p $HOME/.kube
    if [ -f $HOME/.kube/config ]; then
        mv $HOME/.kube/config $HOME/.kube/config.orig
    fi
    sudo KUBECONFIG=/var/lib/microshift/resources/kubeadmin/kubeconfig:$HOME/.kube/config.orig  kubectl config view --flatten > $HOME/.kube/config
}


# Script execution
get_distro
get_arch
if [ $DISTRO = "rhel" ]; then
    register_subs
fi
install_dependencies
install_crio
crio_conf
verify_crio
get_kubectl
get_microshift

until sudo test -f /var/lib/microshift/resources/kubeadmin/kubeconfig
do
     sleep 2
done
prepare_kubeconfig


