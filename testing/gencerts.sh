#!/bin/bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Generates the a CA cert, a server key, and a server cert signed by the CA. 
# reference: https://github.com/kubernetes/kubernetes/blob/master/plugin/pkg/admission/webhook/gencerts.sh
set -e

CN_BASE="kube-graffiti"
outfile="webhook-tls-secret.yaml"

cat > server.conf << EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
EOF

# Create a certificate authority
openssl genrsa -out ca-key.pem 2048
openssl req -x509 -new -nodes -key ca-key.pem -days 100000 -out ca-cert.pem -subj "/CN=${CN_BASE}-ca"

# Create a server certiticate
openssl genrsa -out server-key.pem 2048
# Note the CN is the DNS name of the service of the webhook.
openssl req -new -key server-key.pem -out server.csr -subj "/CN=kube-graffiti.kube-graffiti.svc" -config server.conf
openssl x509 -req -in server.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -days 100000 -extensions v3_req -extfile server.conf

# Add the CA to the server cert chain
cat ca-cert.pem >>server-cert.pem

case $(uname -s) in
   Darwin)
     CODEBASE="darwin"
	 BASE64COMMAND="base64"
     ;;
   Linux)
     CODEBASE="linux"
	 BASE64COMMAND="base64 -w0"
     ;;
   *)
     echo "Sorry! I can't work out your OS... please help and update me."
     exit 1
     ;;
esac

cat > $outfile << EOF
# This file was generated using openssl by the gencerts.sh script
apiVersion: v1
kind: Secret
metadata:
  name: kube-graffiti-certs
  namespace: kube-graffiti
type: Opaque
data:
EOF

for file in ca-cert server-key server-cert; do
	data=$(cat ${file}.pem | $BASE64COMMAND)
	echo "  $file: $data" >> $outfile
done

# Clean up after we're done.
rm *.pem
rm *.csr
rm *.srl
rm *.conf
