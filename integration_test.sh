#!/bin/bash -ue

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

echo "Building kube-graffiti container"
docker build --rm -t ${DEPLOYNAME}:dev .

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
