#!/usr/bin/env bash

### Install the prometheus-operator-kube-stack ###

kubectl apply --server-side -f monitoring/prometheus-operator/crds/
kubectl wait \
	--for condition=Established \
	--all CustomResourceDefinition \
	--namespace=observability
kubectl apply -f monitoring/prometheus-operator/manifests/
