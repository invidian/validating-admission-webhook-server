#!/bin/bash
set -e

minikube start --extra-config=apiserver.feature-gates=CustomResourceSubresources=true --extra-config=apiserver.enable-admission-plugins="Initializers,NamespaceLifecycle,LimitRanger,ServiceAccount,DefaultStorageClass,DefaultTolerationSeconds,NodeRestriction,MutatingAdmissionWebhook,ValidatingAdmissionWebhook,ResourceQuota,PodSecurityPolicy" --kubernetes-version=v1.12.4

kubectl apply -f k8s/cluster-psp/psp.yaml
kubectl auth reconcile -f k8s/cluster-psp/cluster-roles.yaml
kubectl auth reconcile -f k8s/cluster-psp/role-bindings.yaml

kubectl apply -f k8s/validating-admission-webhook/01-namespace.yaml
kubectl apply -f k8s/validating-admission-webhook/02-service.yaml
kubectl apply -f k8s/validating-admission-webhook/03-psp.yaml
kubectl apply -f k8s/validating-admission-webhook/04-config.yaml
kubectl apply -f k8s/validating-admission-webhook/05-deployment.yaml

k8s/validating-admission-webhook/webhook-create-signed-cert.sh
cat k8s/validating-admission-webhook/validatingwebhook.yaml.template | ./k8s/validating-admission-webhook/webhook-patch-ca-bundle.sh > k8s/validating-admission-webhook/06-validatingwebhook.yaml
kubectl apply -f k8s/validating-admission-webhook/06-validatingwebhook.yaml

until kubectl get -n validating-admission-webhook pods | grep validating-admission-webhook | grep Running; do
  sleep 1
done

kubectl apply -f k8s/examples/ || true
