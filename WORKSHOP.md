### Steps
1. Initialize operator SDK (Already done)
```bash
operator-sdk init --domain heureso.com --repo github.com/cloudland-operator-demo/demo-operator
```
2. Create API
```bash
operator-sdk create api --group operator --version v1alpha1 --kind Minio --resource --controller
```
Add the following code to "api/v1alpha1/minio_types.go"
```go
	// User is the user needed to login to Minio UI
	// +kubebuilder:default=MINIO_USER
	// +kubebuilder:validation:Required
	User string `json:"user"`

	// Password is the user password needed to login to Minio UI
	// +kubebuilder:default=MINIO_PASSWORD
	// +kubebuilder:validation:Required
	Password string `json:"password"`

	// ForceRedeploy is any string, modifying this field instructs
	// the Operator to redeploy the Operand
	ForceRedeploy string `json:"forceRedeploy,omitempty"`
```
Run "make generate" to regenerate code after modifying this file
```bash
make generate
 ```
Run "make manifests" to generate a CRD that is based on the API we just defined and creates the RBAC-files
```bash
make manifests
```
3. Add The Deployment Manifests and a simplified way of generating the deployment
Create a directory "assets/manifests" and paste the following
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: minio
    version: latest
    env: dev
  name: minio
  namespace: default
spec:
  progressDeadlineSeconds: 600
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: minio
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: minio
      annotations:
        sidecar.istio.io/inject: "true"
        sidecar.istio.io/rewriteAppHTTPProbers: "true"
    spec:
      containers:
        - env:
            - name: MINIO_ROOT_USER
              value: "admin"
            - name: MINIO_ROOT_PASSWORD
              value: "adminadmin"
          image: quay.io/minio/minio:RELEASE.2022-06-17T02-00-35Z
          args: ["server", "/data", "--console-address", ":9001"]
          name: minio
          ports:
            - containerPort: 9000
              name: mino-api
              protocol: TCP
            - containerPort: 9001
              name: minio-console
              protocol: TCP
          livenessProbe:
            tcpSocket:
              port: 9000
            initialDelaySeconds: 5
            failureThreshold: 1
            periodSeconds: 10
          readinessProbe:
            tcpSocket:
              port: 9001
            initialDelaySeconds: 5
            failureThreshold: 1
            periodSeconds: 10
          resources:
            limits:
              cpu: 500m
              memory: 1Gi
            requests:
              cpu: 125m
              memory: 250M
      restartPolicy: Always
```
Create a file "assets.go" and paste the following content
```go
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
```
Write the control loop






