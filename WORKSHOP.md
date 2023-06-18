## Steps
### 1. Initialize operator SDK (Already done)
```bash
operator-sdk init --domain heureso.com --repo github.com/cloudland-operator-demo/demo-operator
```
### 2. Create API
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
### 3. Add The Deployment Manifests and a simplified way of generating the deployment
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
```go
func (r *MinioReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the operator custom resource
	minioCR := &operatorv1alpha1.Minio{}
	err := r.Get(ctx, req.NamespacedName, minioCR)

	if err != nil && errors.IsNotFound(err) {
		logger.Info("Operator resource object not found.")
		return ctrl.Result{}, nil
	} else if err != nil {
		logger.Error(err, "Error getting operator resource object")

		return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, minioCR)})
	}

	// Read the standard deployment
	deployment := assets.GetDeploymentFromFile("manifests/minio-deployment.yaml")

	// modify deployment according to cr
	deployment.Namespace = req.Namespace
	deployment.Name = req.Name

	_, err = controllerutil.CreateOrPatch(ctx, r.Client, deployment, func() error {
		if minioCR.Spec.User != "" {
			deployment.Spec.Template.Spec.Containers[0].Env[0].Value = minioCR.Spec.User
		}
		if minioCR.Spec.Password != "" {
			deployment.Spec.Template.Spec.Containers[0].Env[1].Value = minioCR.Spec.Password
		}

		return nil
	})

	if err != nil {
		logger.Error(err, "Error updating minio deployment.")
		return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, minioCR)})
	}

	return ctrl.Result{}, nil
}
```
Install the custom resource definition (CRD) into the cluster
```bash
make install
```
Run the operator locally against the cluster
```bash
make run
```
Apply the sample CR from "config/samples/operator_v1apha1_minio.yaml"
```bash
kubectl apply -f config/samples/operator_v1apha1_minio.yaml
```
Check if the pod is being started
```bash
kubectl get pods
```
Forward the port of minio
```bash
k port-forward deployments/minio-sample 9999:9001
```
Accept the port-forward from VsCode
Browser opens and you can log in with your provided Credentials

Delete the custom resource (CR)
```bash
kd config/samples/operator_v1alpha1_minio.yaml
kgp
```
The deployment is not deleted :(

Delete the deployment manually
```bash
kubcetl delete deployment minio-sample
```

### 4. Ownership & Reconciliation loop
Actually control the deployment via Ownership
add the following lines to the code
```go
(...)
  // modify deployment according to cr
deployment.Namespace = req.Namespace
deployment.Name = req.Name

// List the bootstrapOperator in the OwnerReference of the deployment, in order to help garbage collection
ctrl.SetControllerReference(operatorCR, deployment, r.Scheme)
(...)
```
Also modify the setupWithManager Function
```go
// SetupWithManager sets up the controller with the Manager.
func (r *BootstrapOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.BootstrapOperator{}).
		// The operator will also react on changes of deployments it owns (line 98)
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
```
Restart the operator locally with
```bash
make run
```
Apply the CR
```bash
ka config/samples/operator_v1alpha1_minio.yaml
```
Check for the pods
```bash
kgp
```
Delete the CR and observe the deployment being deleted now
```bash
kd config/samples/operator_v1alpha1_minio.yaml
```




