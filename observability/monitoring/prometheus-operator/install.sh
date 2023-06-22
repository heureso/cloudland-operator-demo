#!/usr/bin/env bash

# Replace 'namespace: monitoring' with 'namespace: observability'
# replace 'prometheus-k8s.monitoring.svc' with 'prometheus-k8s.observability.svc'
# Replace 'namespace="monitoring"' with 'namespace="observability"
# Replace '"http://loki.observability.svc.cluster.local"' with '"http://loki-gateway.observability.svc.cluster.local"'
# Replace '"http://tempo.observability.svc.cluster.local/"' with '"http://tempo.observability.svc.cluster.local/"'
# Replace 'namespace=\"observability\"' with 'namespace=\"observability\"'

# Delete prometheus network-policy
# add field 'prometheus.spec.enableRemoteWriteReceiver: true'

kubectl apply --server-side -f crds/
kubectl wait \
	--for condition=Established \
	--all CustomResourceDefinition \
	--namespace=observability
kubectl apply -f manifests/