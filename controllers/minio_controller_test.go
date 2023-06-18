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
