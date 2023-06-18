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
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/cloudland-operator-demo/demo-operator/api/v1alpha1"
	"github.com/cloudland-operator-demo/demo-operator/assets"
)

// MinioReconciler reconciles a Minio object
type MinioReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=operator.heureso.com,resources=minios,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.heureso.com,resources=minios/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.heureso.com,resources=minios/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Minio object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
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

// SetupWithManager sets up the controller with the Manager.
func (r *MinioReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Minio{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
