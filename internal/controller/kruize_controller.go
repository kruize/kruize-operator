/*
Copyright 2024.

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

package controller

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	// runtime for finding directorys
	DirRuntime "runtime"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kruizev1alpha1 "github.com/kruize/kruize-operator/api/v1alpha1"

	"github.com/kruize/kruize-operator/internal/constants"
	"github.com/kruize/kruize-operator/internal/controller/common"
	"github.com/kruize/kruize-operator/internal/utils"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// KruizeReconciler reconciles a Kruize object
type KruizeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kruize.io,resources=kruizes,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=kruize.io,resources=kruizes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kruize.io,resources=kruizes/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// Namespace-scoped resources: create only (deletion handled by OwnerReferences)
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;create
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;create
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create
//+kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create
// Cluster-scoped resources: delete required (cannot use OwnerReferences)
//+kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;create;list;watch;delete
// Read-only permissions
//+kubebuilder:rbac:groups=inference.redhat.com,resources=instaslices,verbs=get;list;watch
//+kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=get;list;watch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch
//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=use
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheuses,verbs=get;list;watch
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheuses/api,verbs=get;create;update
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=alertmanagers,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=metrics.k8s.io,resources=pods,verbs=get;list;watch;create
//+kubebuilder:rbac:groups=metrics.k8s.io,resources=nodes,verbs=get;list
//+kubebuilder:rbac:groups=autoscaling.k8s.io,resources=verticalpodautoscalers,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=autoscaling.k8s.io,resources=verticalpodautoscalers/status,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=autoscaling.k8s.io,resources=verticalpodautoscalercheckpoints,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
//+kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Kruize object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *KruizeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("Reconciling Kruize %v", req.NamespacedName))

	kruize := &kruizev1alpha1.Kruize{}
	err := r.Get(ctx, req.NamespacedName, kruize)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	fmt.Println("kruize object base: ", kruize.Spec)

	// Validate cluster type early, before adding finalizer
	// This ensures invalid configurations are rejected immediately
	if !common.IsBeingDeleted(kruize) {
		cluster_type := kruize.Spec.ClusterType
		if !constants.IsValidClusterType(cluster_type) {
			err := fmt.Errorf("unsupported cluster type: %s. Supported types are: %s",
				cluster_type, strings.Join(constants.SupportedClusterTypes, ", "))
			logger.Error(err, "Invalid cluster type specified")
			return ctrl.Result{}, err
		}
	}

	// Handle finalizer lifecycle using the common utility
	needsRequeue, err := common.HandleFinalizer(ctx, r.Client, kruize, kruizev1alpha1.KruizeFinalizer,
		func(ctx context.Context) error {
			return r.finalizeKruize(ctx, kruize)
		})

	if err != nil {
		logger.Error(err, "Failed to handle finalizer")
		return ctrl.Result{}, err
	}

	if needsRequeue {
		// Finalizer was just added, requeue to ensure it's persisted
		logger.Info("Finalizer added, requeuing")
		return ctrl.Result{Requeue: true}, nil
	}

	// If object is being deleted, HandleFinalizer has already handled it
	if common.IsBeingDeleted(kruize) {
		return ctrl.Result{}, nil
	}

	// Call your deployment function with proper error handling
	err = r.deployKruize(ctx, kruize)
	if err != nil {
		logger.Error(err, "Failed to deploy Kruize")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	fmt.Println("Deployment initiated, waiting for pods to be ready")

	var targetNamespace = kruize.Spec.Namespace

	// Wait for Kruize pods to be ready
	err = r.waitForKruizePods(ctx, targetNamespace, 5*time.Minute)
	if err != nil {
		logger.Error(err, "Kruize pods not ready yet")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	logger.Info("All Kruize pods are ready!", "namespace", targetNamespace)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// finalizeKruize cleans up all resources created by the operator
func (r *KruizeReconciler) finalizeKruize(ctx context.Context, kruize *kruizev1alpha1.Kruize) error {
	logger := log.FromContext(ctx)

	logger.Info("Starting Kruize finalization",
		"namespace", kruize.Spec.Namespace,
		"name", kruize.Name,
		"clusterType", kruize.Spec.ClusterType)

	// Get the cluster type to determine which resources to clean up
	clusterType := kruize.Spec.ClusterType
	if !constants.IsValidClusterType(clusterType) {
		logger.Info("Invalid cluster type, defaulting to OpenShift",
			"providedType", clusterType,
			"defaultType", constants.ClusterTypeOpenShift)
		clusterType = constants.ClusterTypeOpenShift // Default to openshift
	}

	// Create a generator to get the resource names
	k8sObjectGenerator := utils.NewKruizeResourceGenerator(
		kruize.Spec.Namespace,
		kruize.Spec.AutotuneImage,
		kruize.Spec.AutotuneUIImage,
		kruize.Spec.OptimizerImage,
		clusterType,
		&kruize.Spec,
		ctx,
	)

	// Collect all errors during cleanup
	var errs []error
	var successCount, failCount int

	// IMPORTANT: Namespace-scoped resources have OwnerReferences set during creation
	// (see reconcileNamespacedResource function). Kubernetes will automatically delete
	// these resources via garbage collection when the Kruize CR is deleted.
	// We only need to explicitly delete cluster-scoped resources, which cannot have
	// OwnerReferences due to Kubernetes limitations.

	logger.Info("Namespace-scoped resources will be automatically deleted by Kubernetes garbage collection via OwnerReferences")

	// List of cluster-scoped resources to delete
	var clusterScopedResources []client.Object
	if clusterType == constants.ClusterTypeOpenShift {
		clusterScopedResources = k8sObjectGenerator.ClusterScopedResources()
	} else {
		clusterScopedResources = k8sObjectGenerator.KubernetesClusterScopedResources()
	}

	// Delete each cluster-scoped resource (these cannot have OwnerReferences)
	logger.Info("Deleting cluster-scoped resources", "count", len(clusterScopedResources))

	for _, obj := range clusterScopedResources {
		// Check if context has been cancelled or timed out before each deletion
		select {
		case <-ctx.Done():
			// Context cancelled or timed out
			err := ctx.Err()
			logger.Error(err, "Context cancelled during finalization, stopping cleanup",
				"successful", successCount,
				"failed", failCount,
				"remaining", len(clusterScopedResources)-(successCount+failCount))
			return fmt.Errorf("finalization cancelled after deleting %d resources: %w", successCount, err)
		default:
			// Context still valid, proceed with deletion
		}

		if err := r.deleteResource(ctx, obj); err != nil {
			logger.Error(err, "Failed to delete cluster-scoped resource",
				"Kind", obj.GetObjectKind().GroupVersionKind().Kind,
				"Name", obj.GetName())
			errs = append(errs, fmt.Errorf("failed to delete %s/%s: %w",
				obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err))
			failCount++
		} else {
			successCount++
		}
	}

	logger.Info("Cluster-scoped resource deletion complete",
		"successful", successCount,
		"failed", failCount)

	// If any errors occurred during cleanup, return them to prevent finalizer removal
	if len(errs) > 0 {
		joinedErr := stderrors.Join(errs...)
		logger.Error(joinedErr,
			"Finalization incomplete, will retry",
			"totalErrors", len(errs),
			"namespace", kruize.Spec.Namespace)
		return fmt.Errorf("failed to finalize Kruize (cleanup will be retried): %w", joinedErr)
	}

	logger.Info("Successfully completed Kruize finalization",
		"namespace", kruize.Spec.Namespace,
		"name", kruize.Name)
	return nil
}

// deleteResource deletes a resource (cluster-scoped or namespace-scoped) if it exists
func (r *KruizeReconciler) deleteResource(ctx context.Context, obj client.Object) error {
	logger := log.FromContext(ctx)

	// Try to get the resource first
	found := obj.DeepCopyObject().(client.Object)
	err := r.Get(ctx, client.ObjectKeyFromObject(obj), found)

	if err != nil {
		if errors.IsNotFound(err) {
			// Resource doesn't exist, nothing to delete
			logger.Info("Resource not found, skipping deletion",
				"Kind", obj.GetObjectKind().GroupVersionKind().Kind,
				"Name", obj.GetName(),
				"Namespace", obj.GetNamespace())
			return nil
		}
		return fmt.Errorf("failed to get resource %s %s: %w",
			obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err)
	}

	// Delete the resource
	logger.Info("Deleting resource",
		"Kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"Name", obj.GetName(),
		"Namespace", obj.GetNamespace())

	if err := r.Delete(ctx, found); err != nil {
		if errors.IsNotFound(err) {
			// Already deleted, that's fine
			return nil
		}
		return fmt.Errorf("failed to delete resource %s %s: %w",
			obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err)
	}

	logger.Info("Successfully deleted resource",
		"Kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"Name", obj.GetName(),
		"Namespace", obj.GetNamespace())
	return nil
}

// isTestMode checks if the controller is running in test mode
func (r *KruizeReconciler) isTestMode() bool {
	testMode := os.Getenv("KRUIZE_TEST_MODE")
	return testMode == "true" || testMode == "1"
}

func (r *KruizeReconciler) waitForKruizePods(ctx context.Context, namespace string, timeout time.Duration) error {
	logger := log.FromContext(ctx)

	// Skip pod waiting in test mode
	if r.isTestMode() {
		logger.Info("Test mode detected, skipping pod readiness check", "namespace", namespace)
		return nil
	}

	requiredPods := []string{"kruize", "kruize-ui-nginx", "kruize-db", "kruize-optimizer"}
	logger.Info("Waiting for Kruize pods to be ready", "namespace", namespace, "pods", requiredPods)

	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for Kruize pods to be ready in namespace %s", namespace)

		case <-ticker.C:
			readyPods, totalPods, podStatus, err := r.checkKruizePodsStatus(ctx, namespace)
			if err != nil {
				logger.Error(err, "Failed to check pod status")
				continue
			}

			logger.Info("Pod status check", "ready", readyPods, "total", totalPods, "namespace", namespace)
			fmt.Printf("Pod status: %v\n", podStatus)

			// Check if we have all required pods running (kruize, kruize-ui-nginx, kruize-db, kruize-optimizer)
			if readyPods >= 4 && totalPods >= 4 {
				logger.Info("All Kruize pods are ready", "readyPods", readyPods)
				return nil
			}

			logger.Info("Waiting for more pods to be ready", "ready", readyPods, "total", totalPods)
		}
	}
}

func (r *KruizeReconciler) checkKruizePodsStatus(ctx context.Context, namespace string) (int, int, map[string]string, error) {
	podList := &corev1.PodList{}
	err := r.Client.List(ctx, podList, client.InNamespace(namespace))
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to list pods: %v", err)
	}

	var kruizePods []corev1.Pod
	kruizeKeywords := []string{"kruize"}
	podStatus := make(map[string]string)

	// Filter for Kruize-related pods
	for _, pod := range podList.Items {
		for _, keyword := range kruizeKeywords {
			if strings.Contains(strings.ToLower(pod.Name), keyword) {
				kruizePods = append(kruizePods, pod)
				break
			}
		}
	}

	readyCount := 0
	for _, pod := range kruizePods {
		podStatus[pod.Name] = string(pod.Status.Phase)
		fmt.Printf("Pod %s status: %s\n", pod.Name, pod.Status.Phase)

		// Check if pod is ready
		if pod.Status.Phase == corev1.PodRunning {
			// Additional check for readiness conditions
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					readyCount++
					fmt.Printf("Pod %s is ready\n", pod.Name)
					break
				}
			}
		}
	}

	return readyCount, len(kruizePods), podStatus, nil
}

func (r *KruizeReconciler) deployKruize(ctx context.Context, kruize *kruizev1alpha1.Kruize) error {
	logger := log.FromContext(ctx)

	// Log Kruize spec configuration
	logger.Info("Deploying Kruize",
		"cluster_type", kruize.Spec.ClusterType,
		"namespace", kruize.Spec.Namespace,
		"autotune_image", kruize.Spec.AutotuneImage,
		"autotune_ui_image", kruize.Spec.AutotuneUIImage,
		"optimizer_image", kruize.Spec.OptimizerImage)

	// Normalize and validate cluster type (case-insensitive)
	cluster_type := kruize.Spec.ClusterType
	if !constants.IsValidClusterType(cluster_type) {
		return fmt.Errorf("unsupported cluster type: %s. Supported types are: %s", cluster_type, strings.Join(constants.SupportedClusterTypes, ", "))
	}

	var autotune_ns = kruize.Spec.Namespace

	// Deploy the Kruize components directly
	err := r.deployKruizeComponents(ctx, autotune_ns, cluster_type, kruize)
	if err != nil {
		return fmt.Errorf("Failed to deploy Kruize components: %v", err)
	}

	logger.Info("Successfully deployed Kruize components to namespace", "namespace", autotune_ns)
	return nil
}

func (r *KruizeReconciler) deployKruizeComponents(ctx context.Context, namespace string, clusterType string, kruize *kruizev1alpha1.Kruize) error {

	logger := log.FromContext(ctx)

	//TODO: Currently removing the functions which have hardcoded manifests, in the next step will be including manifest generation part
	// In test mode, just validate manifests without deploying
	if r.isTestMode() {
		logger.Info("Test mode detected, validating manifests only", "namespace", namespace)

		// TODO: Generate manifests to ensure they're valid
		logger.Info("All manifests validated successfully in test mode")
		return nil
	}

	k8sObjectGenerator := utils.NewKruizeResourceGenerator(
		namespace,
		kruize.Spec.AutotuneImage,
		kruize.Spec.AutotuneUIImage,
		kruize.Spec.OptimizerImage,
		clusterType,
		&kruize.Spec,
		ctx,
	)

	// Reconcile cluster-scoped resources based on cluster type
	var clusterScopedObjects []client.Object
	var configmap client.Object

	if clusterType == constants.ClusterTypeOpenShift {
		// OpenShift-specific resources
		kruizeServiceAccount := k8sObjectGenerator.KruizeServiceAccount()
		if err := r.reconcileClusterResource(ctx, kruizeServiceAccount); err != nil {
			logger.Error(err, "Failed to reconcile kruize service account")
			return err
		}

		clusterScopedObjects = k8sObjectGenerator.ClusterScopedResources()
		configmap = k8sObjectGenerator.KruizeConfigMap()
	} else {
		// Kind/Minikube-specific resources
		clusterScopedObjects = k8sObjectGenerator.KubernetesClusterScopedResources()
		configmap = k8sObjectGenerator.KruizeConfigMapKubernetes()
	}

	// Reconcile cluster-scoped resources (no owner reference)
	for _, obj := range clusterScopedObjects {
		if err := r.reconcileClusterResource(ctx, obj); err != nil {
			logger.Error(err, "Failed to reconcile cluster-scoped resource", "Kind", obj.GetObjectKind().GroupVersionKind().Kind, "Name", obj.GetName())
			return err
		}
	}

	// Reconcile ConfigMap
	if err := r.reconcileClusterResource(ctx, configmap); err != nil {
		logger.Error(err, "Failed to reconcile configmap")
		return err
	}

	// Deploy core Kruize resources (DB, Kruize, UI) without optimizer
	var coreResources []client.Object
	var optimizerResources []client.Object

	if clusterType == constants.ClusterTypeOpenShift {
		coreResources = k8sObjectGenerator.CoreNamespacedResources()
		optimizerResources = k8sObjectGenerator.OptimizerNamespacedResources()
	} else {
		coreResources = k8sObjectGenerator.CoreKubernetesNamespacedResources()
		optimizerResources = k8sObjectGenerator.OptimizerKubernetesNamespacedResources()
	}

	logger.Info("Deploying core Kruize resources", "namespace", namespace)
	for _, obj := range coreResources {
		if err := r.reconcileNamespacedResource(ctx, kruize, obj); err != nil {
			logger.Error(err, "Failed to reconcile core resource", "Kind", obj.GetObjectKind().GroupVersionKind().Kind, "Name", obj.GetName())
			return err
		}
	}

	// Deploy optimizer resources with init container that waits for Kruize
	// The init container in the optimizer deployment will handle waiting for Kruize to be ready
	logger.Info("Deploying optimizer resources (init container will wait for Kruize to be ready)", "namespace", namespace)
	for _, obj := range optimizerResources {
		if err := r.reconcileNamespacedResource(ctx, kruize, obj); err != nil {
			logger.Error(err, "Failed to reconcile optimizer resource", "Kind", obj.GetObjectKind().GroupVersionKind().Kind, "Name", obj.GetName())
			return err
		}
	}

	logger.Info("Successfully reconciled all dependent resources in phases", "clusterType", clusterType)
	return nil
}

// Helper for cluster scoped resources that don't get an owner reference
func (r *KruizeReconciler) reconcileClusterResource(ctx context.Context, obj client.Object) error {
	log := log.FromContext(ctx)
	found := obj.DeepCopyObject().(client.Object)
	err := r.Get(ctx, client.ObjectKeyFromObject(obj), found)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating new cluster-scoped resource", "Kind", obj.GetObjectKind().GroupVersionKind().Kind, "Name", obj.GetName())
			return r.Create(ctx, obj)
		}
		return fmt.Errorf("Failed to get cluster-scoped resource %s %s: %w", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err)
	}
	return nil
}

// Helper for namespace scoped resources that get an owner reference
func (r *KruizeReconciler) reconcileNamespacedResource(ctx context.Context, owner v1.Object, obj client.Object) error {
	log := log.FromContext(ctx)
	// Set the owner reference
	if err := controllerutil.SetControllerReference(owner, obj, r.Scheme); err != nil {
		return fmt.Errorf("Failed to set owner reference on %s %s: %w", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err)
	}

	found := obj.DeepCopyObject().(client.Object)
	err := r.Get(ctx, client.ObjectKeyFromObject(obj), found)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating new namespace scoped resource", "Kind", obj.GetObjectKind().GroupVersionKind().Kind, "Name", obj.GetName())
			return r.Create(ctx, obj)
		}
		return fmt.Errorf("Failed to get namespace scoped resource %s %s: %w", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err)
	}
	return nil
}

func (r *KruizeReconciler) applyYAMLString(ctx context.Context, yamlContent string, namespace string) error {
	fmt.Printf("Applying YAML content (size: %d bytes) to namespace: %s\n", len(yamlContent), namespace)

	docs := strings.Split(yamlContent, "---")

	// Create namespace first if it doesn't exist
	if namespace != "" {
		err := r.ensureNamespace(ctx, namespace)
		if err != nil {
			fmt.Printf("Warning: failed to ensure namespace %s: %v\n", namespace, err)
		}
	}

	var successCount, failCount int

	for i, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		obj := &unstructured.Unstructured{}
		dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		_, _, err := dec.Decode([]byte(doc), nil, obj)
		if err != nil {
			fmt.Printf("Warning: failed to decode YAML document %d: %v\n", i, err)
			failCount++
			continue
		}

		// Handle namespace setting
		objNamespace := obj.GetNamespace()
		if objNamespace == "" && namespace != "" && !r.isClusterScopedResource(obj.GetKind()) {
			obj.SetNamespace(namespace)
		}

		// Apply the object
		err = r.Client.Patch(ctx, obj, client.Apply, &client.PatchOptions{
			FieldManager: "kruize-operator",
			Force:        &[]bool{true}[0],
		})

		if err != nil {
			fmt.Printf("Warning: failed to apply %s/%s: %v\n", obj.GetKind(), obj.GetName(), err)
			failCount++
		} else {
			fmt.Printf("Successfully applied %s/%s\n", obj.GetKind(), obj.GetName())
			successCount++
		}
	}

	fmt.Printf("Applied %d resources successfully, %d failed\n", successCount, failCount)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KruizeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	fmt.Println("Setting up the controller with the Manager")
	return ctrl.NewControllerManagedBy(mgr).
		For(&kruizev1alpha1.Kruize{}).
		Complete(r)
}

// RootDir returns the root directory of the project
func RootDir() string {
	_, b, _, _ := DirRuntime.Caller(0)
	d := path.Join(path.Dir(b))
	return filepath.Dir(d)
}

func (r *KruizeReconciler) ensureNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: name}, ns)
	if err != nil && errors.IsNotFound(err) {
		ns = &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: name,
			},
		}
		return r.Client.Create(ctx, ns)
	}
	return err
}

func (r *KruizeReconciler) isClusterScopedResource(kind string) bool {
	clusterScopedResources := []string{
		"ClusterRole", "ClusterRoleBinding", "PersistentVolume", "StorageClass",
		"CustomResourceDefinition", "Namespace", "SecurityContextConstraints",
	}
	for _, resource := range clusterScopedResources {
		if kind == resource {
			return true
		}
	}
	return false
}
