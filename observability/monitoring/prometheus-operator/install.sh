#!/usr/bin/env bash

kubectl apply --server-side -f crds/
kubectl wait \
	--for condition=Established \
	--all CustomResourceDefinition \
	--namespace=observability
kubectl apply -f manifests/