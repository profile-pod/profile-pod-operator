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

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	profilepodiov1alpha1 "github.com/profile-pod/profile-pod-operator/api/v1alpha1"
	"github.com/profile-pod/profile-pod-operator/controllers/constants"
)

const podflameFinalizer = "profilepod.io/finalizer"

// PodFlameReconciler reconciles a PodFlame object
type PodFlameReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Clientset         *kubernetes.Clientset
	OperatorNamesapce string
	Recorder          record.EventRecorder
}

var (
	IgnoreStatusChange = builder.WithPredicates(predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Update only if spec / annotations / labels change, ie. ignore status changes
			return (e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()) ||
				!equality.Semantic.DeepEqual(e.ObjectNew.GetAnnotations(), e.ObjectOld.GetAnnotations()) ||
				!equality.Semantic.DeepEqual(e.ObjectNew.GetLabels(), e.ObjectOld.GetLabels())
		},
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	})
)

//+kubebuilder:rbac:groups=profilepod.io,resources=podflames,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=profilepod.io,resources=podflames/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=profilepod.io,resources=podflames/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete;deletecollection
//+kubebuilder:rbac:groups=core,resources=pods/log,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PodFlame object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *PodFlameReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the podflame instance
	// The purpose is check if the Custom Resource for the Kind podflame
	// is applied on the cluster if not we return nil to stop the reconciliation
	podflame := &profilepodiov1alpha1.PodFlame{}
	err := r.Get(ctx, req.NamespacedName, podflame)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("podflame resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get podflame")
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(podflame, podflameFinalizer) {
		log.Info("Adding Finalizer for podflame")
		if ok := controllerutil.AddFinalizer(podflame, podflameFinalizer); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

		if err = r.Update(ctx, podflame); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}

	ispodflameMarkedToBeDeleted := podflame.GetDeletionTimestamp() != nil
	if ispodflameMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(podflame, podflameFinalizer) {
			log.Info("Performing Finalizer Operations for podflame before delete CR")

			// //Let's add here an status "Downgrade" to define that this resource begin its process to be terminated.
			// meta.SetStatusCondition(&podflame.Status.Conditions, metav1.Condition{Type: typeDegradedMemcached,
			// 	Status: metav1.ConditionUnknown, Reason: "Finalizing",
			// 	Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", podflame.Name)})

			// if err := r.Status().Update(ctx, podflame); err != nil {
			// 	log.Error(err, "Failed to update Memcached status")
			// 	return ctrl.Result{}, err
			// }

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			err := r.doFinalizerOperationsForPodflame(ctx, podflame)
			if err != nil {
				log.Error(err, "Failed to perform all operations required before remove the finalizer")
				return ctrl.Result{}, err
			}
			// Re-fetch the memcached Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			// if err := r.Get(ctx, req.NamespacedName, memcached); err != nil {
			// 	log.Error(err, "Failed to re-fetch memcached")
			// 	return ctrl.Result{}, err
			// }

			// meta.SetStatusCondition(&memcached.Status.Conditions, metav1.Condition{Type: typeDegradedMemcached,
			// 	Status: metav1.ConditionTrue, Reason: "Finalizing",
			// 	Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", memcached.Name)})

			// if err := r.Status().Update(ctx, memcached); err != nil {
			// 	log.Error(err, "Failed to update Memcached status")
			// 	return ctrl.Result{}, err
			// }

			log.Info("Removing Finalizer for podflame after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(podflame, podflameFinalizer); !ok {
				log.Error(err, "Failed to remove finalizer for podfalme")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, podflame); err != nil {
				log.Error(err, "Failed to remove finalizer for podflame")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	_, err = r.reconcilePod(ctx, podflame)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// finalizeMemcached will perform the required operations before delete the CR.
func (r *PodFlameReconciler) doFinalizerOperationsForPodflame(ctx context.Context, podflame *profilepodiov1alpha1.PodFlame) error {
	deleteOptions := &metav1.DeleteOptions{}
	listOptions := &metav1.ListOptions{}
	listOptions.LabelSelector = labels.Set(labelsForPodfalme(podflame)).String()
	err := r.Clientset.CoreV1().Pods(r.OperatorNamesapce).DeleteCollection(ctx, *deleteOptions, *listOptions)
	if err != nil {
		return err
	}
	// The following implementation will raise an event
	r.Recorder.Event(podflame, "Warning", "Deleting",
		fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
			podflame.Name,
			podflame.Namespace))
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodFlameReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&profilepodiov1alpha1.PodFlame{}, IgnoreStatusChange).
		Watches(&source.Kind{
			Type: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: r.OperatorNamesapce,
				Labels: map[string]string{
					constants.ManagedBy: constants.OperatorName,
				}}},
		},
			handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
				var result = []reconcile.Request{}
				annotations := a.GetAnnotations()
				podFlameName, nameOk := annotations[constants.AnnotationName]
				podFlameNameSpace, namespaceOk := annotations[constants.AnnotationNamespace]
				if nameOk && namespaceOk {
					result = []reconcile.Request{
						{NamespacedName: types.NamespacedName{
							Name:      podFlameName,
							Namespace: podFlameNameSpace,
						}},
					}
				}
				return result
			}),
		).
		Complete(r)
}
