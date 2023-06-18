#!/usr/bin/env bash

ARCH=$(case $(uname -m) in x86_64) echo -n amd64 ;; aarch64) echo -n arm64 ;; *) echo -n $(uname -m) ;; esac)
OS=$(uname | awk '{print tolower($0)}')

pwd
tmp=$(mktemp -d)
cd "${tmp}"

OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download/v1.28.1
curl -LO ${OPERATOR_SDK_DL_URL}/operator-sdk_${OS}_${ARCH}

gpg --keyserver keyserver.ubuntu.com --recv-keys 052996E2A20B5C7E
curl -LO ${OPERATOR_SDK_DL_URL}/checksums.txt
curl -LO ${OPERATOR_SDK_DL_URL}/checksums.txt.asc
gpg -u "Operator SDK (release) <cncf-operator-sdk@cncf.io>" --verify checksums.txt.asc

grep operator-sdk_${OS}_${ARCH} checksums.txt | sha256sum -c -

chmod +x operator-sdk_${OS}_${ARCH}
sudo mv operator-sdk_${OS}_${ARCH} /usr/local/bin/operator-sdk

sudo apt update
sudo apt install fzf

curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

sudo git clone https://github.com/ahmetb/kubectx /opt/kubectx
sudo ln -s /opt/kubectx/kubectx /usr/local/bin/kubectx
sudo ln -s /opt/kubectx/kubens /usr/local/bin/kubens

echo "alias ns=kubens" >> ~/.bashrc
echo "source <(kubectl completion bash)"  >> ~/.bashrc
echo "alias k=kubectl"  >> ~/.bashrc
echo "complete -F __start_kubectl k"  >> ~/.bashrc
echo "alias kgp='kubectl get pods'"  >> ~/.bashrc
echo "alias ka='kubectl apply -f'"  >> ~/.bashrc
echo "alias kc='kubectl create -f'"  >> ~/.bashrc
echo "alias kd='kubectl delete -f'"  >> ~/.bashrc
