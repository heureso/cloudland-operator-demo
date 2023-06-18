package assets

import (
	"embed"
	v1 "k8s.io/api/core/v1"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	//go:embed manifests/*
	manifests embed.FS

	appsScheme = runtime.NewScheme()
	appsCodecs = serializer.NewCodecFactory(appsScheme)
)

func init() {
	if err := appsv1.AddToScheme(appsScheme); err != nil {
		panic(err)
	}
	if err := v1.AddToScheme(appsScheme); err != nil {
		panic(err)
	}
}

func GetDeploymentFromFile(name string) *appsv1.Deployment {
	deploymentBytes, err := manifests.ReadFile(name)
	if err != nil {
		panic(err)
	}

	deploymentObject, err := runtime.Decode(appsCodecs.UniversalDecoder(appsv1.SchemeGroupVersion), deploymentBytes)
	if err != nil {
		panic(err)
	}

	return deploymentObject.(*appsv1.Deployment)
}

func GetServiceFromFile(name string) *v1.Service {
	serviceBytes, err := manifests.ReadFile(name)
	if err != nil {
		panic(err)
	}

	serviceObject, err := runtime.Decode(appsCodecs.UniversalDecoder(v1.SchemeGroupVersion), serviceBytes)
	if err != nil {
		panic(err)
	}

	return serviceObject.(*v1.Service)
}
