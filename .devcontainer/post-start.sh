#!/usr/bin/env bash

if k3d cluster list | grep "k3s-default" ; then
    k3d cluster start
else
    k3d cluster create -p "8000:80@loadbalancer" --registry-create registry.localhost:localhost:5000
fi

mkdir -p ~/.kube
k3d kubeconfig get k3s-default > ~/.kube/config
