# Kubernetes configurable validating admission webhook server [![Build Status](https://travis-ci.com/invidian/validating-admission-webhook-server.svg?branch=master)](https://travis-ci.com/invidian/validating-admission-webhook-server)

This repository contains source code for configurable Kubernetes validating admission webhook server. Currently only `PodSecurityPolicy` can be validated, but server can be easily extended to validate more kinds of objects.

Currently, only `CREATE` and `UPDATE` operations are supported for validation.

## Table of contents
* [Quick start](#quick-start)
* [Configuring validation rules](#configuring-validation-rules)
* [Configuration examples](#configuration-examples)
* [Testing with minikube](#testing-with-minikube)
* [Building](#building)
* [Deploying](#deploying)
* [Testing](#testing)
* [Future improvements](#future-improvements)
* [Compatibility](#compatibility)
* [Extending validator functionality](#extending-validator-functionality)
* [References](#references)
* [Authors](#authors)

## Quick start

If you don't want to go through all setup steps manually, there is a handy `run.sh` script, which simply contains all steps required for testing. Remember to examine the script before running it.

It is recommended to run `minikube delete` before executing the script.

## Configuring validation rules

This validator use [JSONPath](https://kubernetes.io/docs/reference/kubectl/jsonpath/) query syntax to extract data from validated object, then it checks if the output matches optional regular expression. If there is no regular expression defined for the rule, validation will fail if query returns any output.

Configuration is done through [config.yaml](https://github.com/invidian/validating-admission-webhook-server/blob/master/k8s/validating-admission-webhook/04-config.yaml) configuration file.

By default, validator is configured with following [rules](https://github.com/invidian/validating-admission-webhook-server/blob/master/k8s/validating-admission-webhook/04-config.yaml):
```
---
kinds:
  - name: "PodSecurityPolicy"
    rules:
      - name: 'Reject seccomp unconfined'
        jsonpath: "{.metadata.annotations['seccomp\\.security\\.alpha\\.kubernetes\\.io/defaultProfileName', 'seccomp\\.security\\.alpha\\.kubernetes\\.io/allowededProfileNames']}"
        regexp: '(unconfined|\*|^$)'
        message: 'Creating PodSecurityPolicy which allows seccomp to be disabled is not allowed'
```

This configuration will reject any `PodSecurityPolicy` objects, which allows seccomp to be disabled.

Rule object accepts following parameters:
* name - name of the rule, used for logging
* jsonpath - JSONPath query used for extracting data from validated objects
* regexp - *optional* Regular expression, which is executed on output returned from JSONPath query
* message - User friendly error message

## Configuration examples

* To reject objects without label `foo`:
```
- name: "Require label foo"
  jsonpath: "{.metadata.labels.foo}"
  regexp: "^$"
  message: "Label foo is required"
```
* To reject objects which has label `foo` and value `bar`:
```
- name: "Label foo can't have bar"
  jsonpath: "{.metadata.namespace.foo}"
  regexp: "^$"
  message: "Label foo cannot have value 'bar'"
```

See [validator_test.go](https://github.com/invidian/validating-admission-webhook-server/blob/master/validator_test.go) for more examples.

## Testing with minikube

In order to test this on cluster created with [minikube](https://github.com/kubernetes/minikube), `minikube` needs to be started with following flags:
  - `--extra-config=apiserver.enable-admission-plugins="Initializers,NamespaceLifecycle,LimitRanger,ServiceAccount,DefaultStorageClass,DefaultTolerationSeconds,NodeRestriction,MutatingAdmissionWebhook,ValidatingAdmissionWebhook,ResourceQuota,PodSecurityPolicy"` - Contains default admission plugins enabled for minikube cluster with additional PodSecurityPolicy. Without this, PodSecurityPolicy won't be validated.
  - `--extra-config=apiserver.feature-gates=CustomResourceSubresources=true`

In addition, this has been tested with Kubernetes v1.12.4, so adding `--kubernetes-version=v1.12.4` flag is recommended.

Example command to start `minikube`:
```
minikube start --extra-config=apiserver.feature-gates=CustomResourceSubresources=true --extra-config=apiserver.enable-admission-plugins="Initializers,NamespaceLifecycle,LimitRanger,ServiceAccount,DefaultStorageClass,DefaultTolerationSeconds,NodeRestriction,MutatingAdmissionWebhook,ValidatingAdmissionWebhook,ResourceQuota,PodSecurityPolicy" --kubernetes-version=v1.12.4
```

Once `minikube` says, that cluster is running, we need to create initial PodSecurityPolicy, otherwise no pods will be spawned. This can be done by executing following commands:
```
kubectl apply -f k8s/cluster-psp/psp.yaml
kubectl auth reconcile -f k8s/cluster-psp/cluster-roles.yaml
kubectl auth reconcile -f k8s/cluster-psp/role-bindings.yaml
```

Once this is done, `kubectl get pods -n kube-system` should return cluster pods:
```
$ kubectl get pods -n kube-system
NAME                                    READY     STATUS    RESTARTS   AGE
kube-dns-86f4d74b45-tgwrc               3/3       Running   0          3m
kube-proxy-42lrc                        1/1       Running   0          3m
kubernetes-dashboard-5498ccf677-mp25g   1/1       Running   0          3m
storage-provisioner                     1/1       Running   3          4m
```

See https://github.com/appscodelabs/tasty-kube/tree/master/minikube/1.10/psp for more details about running minikube cluster with PodSecurityPolicy enabled.

## Building

Building should be done with Docker.

If you want to build in `minikube` environment, run following commands:
```
# Build in minikube docker environment, so cluster can fetch an image.
eval $(minikube docker-env)
docker build -t validating-admission-webhook .
```

Then change `k8s/validating-admission-webhook/05-deployment.yaml` to use just `validating-admission-webhook` as an image.

## Deploying

`k8s/validating-admission-webhook` directory contains example deployment files, which can be used for testing.

Execute following command to run it:
```
# Create base objects
kubectl apply -f k8s/validating-admission-webhook/01-namespace.yaml
kubectl apply -f k8s/validating-admission-webhook/02-service.yaml
kubectl apply -f k8s/validating-admission-webhook/03-psp.yaml
kubectl apply -f k8s/validating-admission-webhook/04-config.yaml
kubectl apply -f k8s/validating-admission-webhook/05-deployment.yaml

# Execute webhook-create-signed-cert.sh script to generate signed certificate
# for our deployment, as validate requests must use HTTPS.
k8s/validating-admission-webhook/webhook-create-signed-cert.sh

# Once we have signed certificate, we can generate validating webhook objects
# from the template and apply it.
cat k8s/validating-admission-webhook/validatingwebhook.yaml.template | ./k8s/validating-admission-webhook/webhook-patch-ca-bundle.sh > k8s/validating-admission-webhook/06-validatingwebhook.yaml
kubectl apply -f k8s/validating-admission-webhook/06-validatingwebhook.yaml
```

At this point, webhook server should be running. You can check it with:
```
$ kubectl get -n validating-admission-webhook pods
NAME                                            READY     STATUS    RESTARTS   AGE
validating-admission-webhook-6846859c5f-lgz8n   1/1       Running   0          14h
```

See https://banzaicloud.com/blog/k8s-admission-webhooks/ and https://github.com/banzaicloud/admission-webhook-example for more details.

## Testing
In `k8s/examples` directory, you can find example PodSecurityPolicy objects, which can be used for verifying, that validator is working as expected.
Here is the output from example test:
```
$ kubectl apply -f k8s/examples/
podsecuritypolicy.policy "good-psp" configured
Error from server: error when creating "k8s/examples/bad-psp-default.yaml": admission webhook "validating-admission-webhook.yourdomain.com" denied the request: Creating PodSecurityPolicy which allows seccomp to be disabled is not allowed
Error from server: error when creating "k8s/examples/bad-psp-explicit.yaml": admission webhook "validating-admission-webhook.yourdomain.com" denied the request: Creating PodSecurityPolicy which allows seccomp to be disabled is not allowed
Error from server: error when creating "k8s/examples/bad-psp-profiles-wildcard.yaml": admission webhook "validating-admission-webhook.yourdomain.com" denied the request: Creating PodSecurityPolicy which allows seccomp to be disabled is not allowed
Error from server: error when creating "k8s/examples/bad-psp-profiles.yaml": admission webhook "validating-admission-webhook.yourdomain.com" denied the request: Creating PodSecurityPolicy which allows seccomp to be disabled is not allowed
```

For validating existing cluster objects, following shell loop can be used:
```
for i in $(kubectl get psp -o "jsonpath={.items[*].metadata.name}"); do
  kubectl get psp $i -o "jsonpath={.metadata.annotations['seccomp\.security\.alpha\.kubernetes\.io/defaultProfileName', 'seccomp\.security\.alpha\.kubernetes\.io/allowededProfileNames']}{\"\n\"}" | grep -E '(unconfined|\*|^$)' && echo "PodSecurityPolicy $i invalid!"
done
```

Example output:
```
PodSecurityPolicy default invalid!

PodSecurityPolicy privileged invalid!
```

## Future improvements

Currently, [JSONPath](https://kubernetes.io/docs/reference/kubectl/jsonpath/) syntax is not a full implementation of JSONPath. With full implementation, negation and filter arrays could be used to avoid using additional regular expressions for validating objects. This would also simplify testing, as queries would be compatible with `kubectl get <kind> -o jsonpath"<query>""` output.

Rather than specifying:
```
{.metadata.annotations['seccomp\\.security\\.alpha\\.kubernetes\\.io/defaultProfileName', 'seccomp\\.security\\.alpha\\.kubernetes\\.io/allowededProfileNames']}
```
you would specify it like:
```
{.items[?(@.metadata.annotations['seccomp\\.security\\.alpha\\.kubernetes\\.io/defaultProfileName']=='unconfined', ?(@.metadata.annotations['seccomp\\.security\\.alpha\\.kubernetes\\.io/allowededProfileNames'] !~ /(unconfined|\*|^$)/]}
```

See https://github.com/kubernetes/website/issues/7853 for more details.

## Compatibility

This project has been tested in following environment:
- OS: `Linux 4.20.2-arch1-1-ARCH #1 SMP PREEMPT Sun Jan 13 17:49:00 UTC 2019 x86_64 GNU/Linux`
- minikube version: `v0.32.0`
- Kubernetes version: `v1.12.4`

It should work with any v1.12.x Kubernetes version. For running with other versions, version of `k8s.io/client-go` should be set accordingly in `glide.yaml` file. You can find compatibility matrix [here](https://github.com/kubernetes/client-go#compatibility-matrix).

## Extending validator functionality

To add support for validating more kinds of objects, add:
- proper handling of new kind in `validate` method (`switch req.Kind.Kind`) in `webhook.go` file
- tests related to new kind to `webhook_test.go` file

## References

- [Basics about admission controllers](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
- [Basics about PodSecurityPolicy](https://kubernetes.io/docs/concepts/policy/pod-security-policy/)
- [go-client](https://github.com/kubernetes/client-go)
- [Glide with go-client example](https://github.com/avast/k8s-admission-webhook)
- [Good article about writing admission webhooks](https://banzaicloud.com/blog/k8s-admission-webhooks/)
- [Example admission webhook](https://github.com/banzaicloud/admission-webhook-example)
- [Minikube with PodSecurityPolicy](https://github.com/appscodelabs/tasty-kube/tree/master/minikube/1.10/psp)
- [Usage of Kubernetes JSONPath API](https://github.com/kubernetes/client-go/blob/v9.0.0/util/jsonpath/jsonpath_test.go)

## Authors
* **Mateusz Gozdek** - *Initial work* - [invidian](https://github.com/invidian)
