#!/bin/bash -ue

# Copyright (C) 2018 Expedia Group.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

NAMESPACE=kube-graffiti
DEPLOYNAME=kube-graffiti

minikube_running() {
  minikube status | head -1 | awk '{print $2}' | grep -q -e "Running"
}

download_kubectl() {
  local binname=$1
  local version=$2
  local codebase=$3

  echo "Downloading kubectl for ${codebase}..."
  echo "curl -L -k https://storage.googleapis.com/kubernetes-release/release/${version}/bin/${codebase}/amd64/${binname}"
  curl -f -L -k https://storage.googleapis.com/kubernetes-release/release/${version}/bin/${codebase}/amd64/${binname} >${binname}
  chmod a+x ${binname}
}

kubectlbin="kubectl"
case $(uname -s) in
   Darwin)
     codebase="darwin"
     ;;
   Linux)
     codebase="linux"
     ;;
   CYGWIN*|MINGW32*|MSYS*)
     codebase="windows"
     kubectlbin="kubectl.exe"
     ;;
   *)
     echo "Can't detect the OS"
     ;;
esac

latest_k8s_version=$(minikube get-k8s-versions | head -2 | tail -1 | awk '{print $2}')
if set_version=$(minikube config get kubernetes-version 2> /dev/null); then
  echo "Deploying kubernetes version: $set_version"
  if [[ "$set_version" != "${latest_k8s_version}" ]]; then
    echo "Warning! There is a newer version of kubernetes available: ${latest_k8s_version}"
    echo "Use minikube config unset kubernetes-version to reset config and pick up newest version."
  fi
else
  echo "No kubernetes version configured, setting the latest: ${latest_k8s_version}"
  minikube config set kubernetes-version ${latest_k8s_version}
  if minikube_running; then
    echo "Stopping minikube"
    minikube stop
  fi
fi 

if ! minikube_running; then
  echo "Starting minikube"
  minikube start
fi

eval $(minikube docker-env)
echo "Building kube-graffiti container"
docker build --rm -t ${DEPLOYNAME}:dev .

if [[ ! -f "./${kubectlbin}" ]]; then
  [[ "${codebase}" == "" ]] && echo "No kubectl and can't work out codebase to download" && exit 1
  # Get kubectl that matches chosen k8s version or default to the latest version
  kubectl_version=$(minikube config get kubernetes-version)
  download_kubectl ${kubectlbin} ${kubectl_version} ${codebase}
fi

for template in namespace roles serviceaccount rolebindings service webhook-tls-secret configmap
do
  [[ -f "testing/${template}.yaml" ]] && ./$kubectlbin apply -f testing/${template}.yaml
done
eval $(minikube docker-env)
./${kubectlbin} -n ${NAMESPACE} get deploy ${DEPLOYNAME} && ./${kubectlbin} -n ${NAMESPACE} delete deployment ${DEPLOYNAME}
./$kubectlbin create -f testing/deploy.yaml
