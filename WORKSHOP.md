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
### 5. Conditions
Add a condition field to the MinioStatus struct
```go
// MinioStatus defines the observed state of Minio
type MinioStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Conditions []metav1.Condition `json:"conditions"`
}
```
Add some printer columns for better readability
```go
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description="" 
```

Generate the new CRD and install it into the K8s cluster
```bash
make generate
make manifests
make install
```

Define Conditions in the error handling
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
		meta.SetStatusCondition(&minioCR.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             operatorv1alpha1.ReasonCRNotAvailable,
			LastTransitionTime: metav1.NewTime(time.Now()),
			Message:            fmt.Sprintf("unable to get operator custom resource: %s", err.Error()),
		})
		return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, minioCR)})
	}

	// Read the standard deployment
	deployment := assets.GetDeploymentFromFile("manifests/minio-deployment.yaml")

	// modify deployment according to cr
	deployment.Namespace = req.Namespace
	deployment.Name = req.Name

	// List the bootstrapOperator in the OwnerReference of the deployment, in order to help garbage collection
	ctrl.SetControllerReference(minioCR, deployment, r.Scheme)

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
		meta.SetStatusCondition(&minioCR.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             operatorv1alpha1.ReasonOperandDeploymentFailed,
			LastTransitionTime: metav1.NewTime(time.Now()),
			Message:            fmt.Sprintf("unable to update operand deployment: %s", err.Error()),
		})
		return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, minioCR)})
	}

	meta.SetStatusCondition(&minioCR.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             operatorv1alpha1.ReasonSucceeded,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            "operator successfully reconciling",
	})

	return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, minioCR)})
}

```bash

Restart the operator locally via
```go
make run
```

### 6. Write a simple Test

Replace `controllers/suite_test.go` with the following:
```go
/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"k8s.io/apimachinery/pkg/util/rand"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/cloudland-operator-demo/demo-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = operatorv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	By("creating controller")
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		MetricsBindAddress:     ":8082",
		HealthProbeBindAddress: ":8083",
	})
	Expect(err).NotTo(HaveOccurred())

	err = (&MinioReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred(), "failed to run manager")
	}()

})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	return string(b)
}

```

Replace the last `meta.setConditionStatus` statement in the `Reconcile`-Method with to following:
```go
	availableCond := getDeploymentCondition(deployment.Status.Conditions, appsv1.DeploymentAvailable)
	var status metav1.ConditionStatus
	if availableCond == nil {
		meta.SetStatusCondition(&minioCR.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionUnknown,
			Reason:             operatorv1alpha1.ReasonDeploymentNotAvailable,
			LastTransitionTime: metav1.NewTime(time.Now()),
			Message:            "operator reconciling",
		})
	} else {
		status = metav1.ConditionStatus(availableCond.Status)
		if status == metav1.ConditionTrue {
			meta.SetStatusCondition(&minioCR.Status.Conditions, metav1.Condition{
				Type:               "Ready",
				Status:             status,
				Reason:             operatorv1alpha1.ReasonSucceeded,
				LastTransitionTime: metav1.NewTime(time.Now()),
				Message:            "operator successfully reconciling",
			})
		} else {
			meta.SetStatusCondition(&minioCR.Status.Conditions, metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionStatus(availableCond.Status),
				Reason:             operatorv1alpha1.ReasonDeploymentNotAvailable,
				LastTransitionTime: metav1.NewTime(time.Now()),
				Message:            "operator reconciling",
			})
		}
	}
```

Add the function `getDeploymentCondition` to `controllers/minio_controller.go`:
```go
func getDeploymentCondition(conditions []appsv1.DeploymentCondition,
	conditionType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}

	return nil
}
```

Create a test which mocks the deployment and asserts the behaviour of our reconciliation loop to reflect the availability of the deployment:
create a new file `controllers/minio_controller_test.go` with the following contents:
```go
package controllers

import (
	"fmt"
	"time"

	operatorv1alpha1 "github.com/cloudland-operator-demo/demo-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Minio Controller", func() {
	var namespace *v1.Namespace
	var minio *operatorv1alpha1.Minio

	BeforeEach(func() {
		namespace = &v1.Namespace{}
		namespace.Name = fmt.Sprintf("test-%s", randStringRunes(5))
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, minio)).To(Succeed())
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
	})

	Context("Creating a minio instance", func() {
		It("should result in Ready = true", func() {
			minio = &operatorv1alpha1.Minio{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: namespace.Name,
				},
				Spec: operatorv1alpha1.MinioSpec{
					User:     "admin",
					Password: "supersecret1234",
				},
			}
			// create a test instance of the minio crd
			Expect(k8sClient.Create(ctx, minio)).To(Succeed())

			// wait for the deployment to appear
			deployment := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test", Namespace: namespace.Name}, deployment)

				return err == nil && deployment.Name == "test"
			}, time.Second*10, time.Second).Should(BeTrue())

			// mock the deployments behaviour by modifying its conditions, so our reconciliation loop is triggered by the fired event
			deployment.Status.Conditions = append(deployment.Status.Conditions, appsv1.DeploymentCondition{
				Type:               appsv1.DeploymentAvailable,
				Status:             v1.ConditionTrue,
				LastUpdateTime:     metav1.NewTime(time.Now()),
				LastTransitionTime: metav1.NewTime(time.Now()),
				Reason:             "MinimumReplicasAvailable",
				Message:            "Deployment has minimum availability.",
			})

			Expect(k8sClient.Status().Update(ctx, deployment)).To(Succeed())

			// assert the minio cr becomes ready
			Eventually(func() bool {
				Expect(k8sClient.Get(ctx,
					types.NamespacedName{Name: "test", Namespace: namespace.Name}, minio,
				)).To(Succeed())

				return meta.IsStatusConditionTrue(minio.Status.Conditions, "Ready")
			}, time.Second*10, time.Second).Should(BeTrue())

			var dl appsv1.DeploymentList
			Expect(k8sClient.List(ctx, &dl, client.InNamespace(namespace.Name)))

			fmt.Println(dl)
		})
	})
})
```

### 7. Docker build - deploy to k8s (with rbac!)
We have to tell Kubebuilder which access roles to assign
Replace in `controllers/minio_controller.go` the following lines:
```go
//+kubebuilder:rbac:groups=operator.heureso.com,resources=minios,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.heureso.com,resources=minios/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.heureso.com,resources=minios/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
```

Stop the locally running operator in case it is still running.

Regenerate the manifests, as we added an annotation
```bash
make manifests
```

We want to push the image to a local registry provided by k3d.
Modify the make-file in line 50 as follows:
```bash
IMG ?= registry.localhost:5000/controller:latest
```

Add the assets to the Dockerfile:
```bash
COPY assets/ assets/
```

Then trigger the build via make:
```bash
make docker-build
```

Then push the generated docker image to the local repository
```bash
make docker-push
```

Deploy the operator to the local kubernetes cluster.
```bash
make deploy
```

Apply the minio CR:
```bash
ka config/samples/operator_v1alpha1_minio.yaml
```

Check the cr and deployment of minio:
```bash
k -n cloudland-operator-demo-system get minios.operator.heureso.com
k -n cloudland-operator-demo-system get pods 
```

### 8. Healthchecks
Basic healthchecks for the operator were already added by scaffolding the project.

See `main.go` line 101-108:
```go
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
```

Instead of `healthz.Ping` you could implement your own functions if extended functionality is needed. They just need to implement this contract:

```go
func(req *http.Request) error
````

Scaffolding also added a flag on which port to bind the listener responding to the healthchecks. See `main.go` line 56:

```go
flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
````

To test this, just run the operator locally and curl the endpoint:
```bash
make run
```

In another terminal window:
```bash
curl localhost:8081/healthz
curl localhost:8081/readyz
```


Healthchecks for the operand were basically delegated to the deployment controller. See 06-test

### 9. Metrics

Create a folder `controller/metrics` and inside it a file `metrics.go`

Create a new metrics registry and register a new metric `minio_reconciles_total`
```go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ReconcilesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "minio_reconciles_total",
			Help: "Number of total reconciliation attempts",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(ReconcilesTotal)
}
```

Increment the created metric in the reconciliation loop.
```go
	// Count the reconcile attempts
	metrics.ReconcilesTotal.Inc()
```

Install the observability stack
```bash
cd observability
./install.sh
```

Tell kubebuilder to create a serviceMonitor for the operator.
Uncomment the following line in `config/default/kustomization.yaml`
```yaml
# [PROMETHEUS] To enable prometheus monitor, uncomment all sections with 'PROMETHEUS'.
- ../prometheus
```

Add an ImagePullPolicy Always, so that the new image gets pulled.
Add the following line to `assets/manifests/minio-deployment.yaml`
```yaml
(...)
image: quay.io/minio/minio:RELEASE.2022-06-17T02-00-35Z
imagePullPolicy: Always
(...)
```

Rebuild the Operator and deploy it again
```bash
cd ..
make docker-build
make docker-push
k -n cloudland-operator-demo-system delete deployments.apps cloudland-operator-demo-controller-manager 
make deploy
```
Delete the minio custom resource and create it again
```bash
k delete minios.operator.heureso.com minio-sample
ka config/samples/operator_v1alpha1_minio.yaml
```

Navigate to prometheus ui in the bowser or port-forward it
```bash
k -n observability port-forward prometheus-k8s-0 9090:9090
```

To generate basic grafana dashboards, use the kubebuilder grafana plugin:

```bash
operator-sdk edit --plugins grafana.kubebuilder.io/v1-alpha
````

This creates a folder `grafana` with dashboards using the controller runtime default metrics.

To generate a dashboard for our custom metric, update the file `grafana/custom-metrics/config.yaml` with the following:
```yaml
customMetrics:
  - metric: minio_reconciles_total
    type: counter
    unit: none
````

Then run the grafana plugin again to generate another dashboard in `grafana/custom-metrics/`:

```bash
operator-sdk edit --plugins grafana.kubebuilder.io/v1-alpha
````

Port-forward grafana and import the dashboards:
```bash
k port-forward -n observability svc/grafana 3000:3000
```

