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
	"os"
	"time"
	"fmt"

	"crypto/tls"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kruizev1alpha1 "github.com/kruize/kruize-operator/api/v1alpha1"
	"github.com/kruize/kruize-operator/internal/constants"
	"github.com/kruize/kruize-operator/internal/utils"
)

// getTestContext returns a context for use in tests
func getTestContext() context.Context {
	return context.Background()
}

// Helper functions to create test specs with custom resource configurations

// createKruizeAppConfig creates a KruizeAppConfig with the given resource values
func createKruizeAppConfig(cpuRequest, cpuLimit, memoryRequest, memoryLimit string) *kruizev1alpha1.KruizeAppConfig {
	config := &kruizev1alpha1.KruizeAppConfig{
		Resources: &kruizev1alpha1.KubernetesResourceRequirements{},
	}
	
	if cpuRequest != "" || memoryRequest != "" {
		config.Resources.Requests = &kruizev1alpha1.ResourceList{}
		if cpuRequest != "" {
			config.Resources.Requests.CPU = cpuRequest
		}
		if memoryRequest != "" {
			config.Resources.Requests.Memory = memoryRequest
		}
	}
	
	if cpuLimit != "" || memoryLimit != "" {
		config.Resources.Limits = &kruizev1alpha1.ResourceList{}
		if cpuLimit != "" {
			config.Resources.Limits.CPU = cpuLimit
		}
		if memoryLimit != "" {
			config.Resources.Limits.Memory = memoryLimit
		}
	}
	
	return config
}

// createKruizeDBConfig creates a KruizeDBConfig with the given resource values
func createKruizeDBConfig(cpuRequest, cpuLimit, memoryRequest, memoryLimit string) *kruizev1alpha1.KruizeDBConfig {
	config := &kruizev1alpha1.KruizeDBConfig{
		Resources: &kruizev1alpha1.KubernetesResourceRequirements{},
	}
	
	if cpuRequest != "" || memoryRequest != "" {
		config.Resources.Requests = &kruizev1alpha1.ResourceList{}
		if cpuRequest != "" {
			config.Resources.Requests.CPU = cpuRequest
		}
		if memoryRequest != "" {
			config.Resources.Requests.Memory = memoryRequest
		}
	}
	
	if cpuLimit != "" || memoryLimit != "" {
		config.Resources.Limits = &kruizev1alpha1.ResourceList{}
		if cpuLimit != "" {
			config.Resources.Limits.CPU = cpuLimit
		}
		if memoryLimit != "" {
			config.Resources.Limits.Memory = memoryLimit
		}
	}
	
	return config
}

// createPVSpec creates a PersistentVolumeSpec with the given values
func createPVSpec(storage, storageClassName, hostPath string, accessModes []kruizev1alpha1.PersistentVolumeAccessMode) *kruizev1alpha1.PersistentVolumeSpec {
	spec := &kruizev1alpha1.PersistentVolumeSpec{}
	
	if storage != "" {
		spec.Capacity = &kruizev1alpha1.StorageCapacity{
			Storage: storage,
		}
	}
	
	if storageClassName != "" {
		spec.StorageClassName = storageClassName
	}
	
	if hostPath != "" {
		spec.HostPath = &kruizev1alpha1.HostPathVolumeSource{
			Path: hostPath,
		}
	}
	
	if len(accessModes) > 0 {
		spec.AccessModes = accessModes
	}
	
	return spec
}

// createPVCSpec creates a PersistentVolumeClaimSpec with the given values
func createPVCSpec(storage string) *kruizev1alpha1.PersistentVolumeClaimSpec {
	if storage == "" {
		return nil
	}
	
	return &kruizev1alpha1.PersistentVolumeClaimSpec{
		Resources: &kruizev1alpha1.PVCResourceRequirements{
			Requests: &kruizev1alpha1.StorageCapacity{
				Storage: storage,
			},
		},
	}
}

// findTypedResource is a helper function to find a specific resource by type, name, and optionally by label
func findTypedResource[T client.Object](resources []client.Object, name string, labelKey string, labelValue string) T {
	var zero T
	for _, resource := range resources {
		if typed, ok := resource.(T); ok {
			if typed.GetName() == name {
				// If no label key is provided, match by name only
				if labelKey == "" {
					return typed
				}
				// Otherwise, match by both name and label
				if labels := typed.GetLabels(); labels != nil && labels[labelKey] == labelValue {
					return typed
				}
			}
		}
	}
	return zero
}

// findContainerByName is a helper function to find a specific container by name in a container list
func findContainerByName(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}

// Helper function to find a Deployment by name in a list of resources
func findDeployment(resources []client.Object, name string) *appsv1.Deployment {
	for _, resource := range resources {
		if resource.GetObjectKind().GroupVersionKind().Kind == "Deployment" && resource.GetName() == name {
			if deployment, ok := resource.(*appsv1.Deployment); ok {
				return deployment
			}
		}
	}
	return nil
}

// Helper function to find a PersistentVolume in a list of resources
func findPersistentVolume(resources []client.Object) *corev1.PersistentVolume {
	for _, resource := range resources {
		if resource.GetObjectKind().GroupVersionKind().Kind == "PersistentVolume" {
			if pv, ok := resource.(*corev1.PersistentVolume); ok {
				return pv
			}
		}
	}
	return nil
}

// Helper function to find a PersistentVolumeClaim in a list of resources
func findPersistentVolumeClaim(resources []client.Object) *corev1.PersistentVolumeClaim {
	for _, resource := range resources {
		if resource.GetObjectKind().GroupVersionKind().Kind == "PersistentVolumeClaim" {
			if pvc, ok := resource.(*corev1.PersistentVolumeClaim); ok {
				return pvc
			}
		}
	}
	return nil
}

// Helper function to get the first container from a deployment and verify it exists
func getContainer(deployment *appsv1.Deployment, expectedName string) *corev1.Container {
	Expect(deployment).NotTo(BeNil(), "Deployment should exist")
	Expect(deployment.Spec.Template.Spec.Containers).NotTo(BeEmpty(), "Deployment should have containers")
	container := &deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal(expectedName), "Container name should match")
	return container
}

var _ = Describe("Kruize Controller", func() {
	ctx := context.Background()

	//setting test mode for the controller
	BeforeEach(func() {
		os.Setenv("KRUIZE_TEST_MODE", "true")
	})

	AfterEach(func() {
		os.Unsetenv("KRUIZE_TEST_MODE")
	})
	Context("Test mode behavior", func() {
		var reconciler *KruizeReconciler

		BeforeEach(func() {
			reconciler = &KruizeReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
		})

		It("should detect test mode correctly", func() {
			os.Setenv("KRUIZE_TEST_MODE", "true")
			Expect(reconciler.isTestMode()).To(BeTrue())

			os.Setenv("KRUIZE_TEST_MODE", "false")
			Expect(reconciler.isTestMode()).To(BeFalse())

			os.Unsetenv("KRUIZE_TEST_MODE")
			Expect(reconciler.isTestMode()).To(BeFalse())
		})

		It("should skip pod waiting in test mode", func() {
			os.Setenv("KRUIZE_TEST_MODE", "true")

			// This should return immediately without error
			err := reconciler.waitForKruizePods(ctx, "test-namespace", time.Second*1)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("When reconciling different cluster types", func() {
		DescribeTable("should handle supported cluster types",
			func(clusterType, namespace, testName string) {
				kruize := &kruizev1alpha1.Kruize{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testName,
						Namespace: "default",
					},
					Spec: kruizev1alpha1.KruizeSpec{
						Cluster_type:      clusterType,
						Namespace:         namespace,
						Autotune_image:    constants.GetDefaultAutotuneImage(),
						Autotune_ui_image: constants.GetDefaultUIImage(),
					},
				}
				Expect(k8sClient.Create(ctx, kruize)).To(Succeed())

				defer func() {
					Expect(k8sClient.Delete(ctx, kruize)).To(Succeed())
				}()

				controllerReconciler := &KruizeReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testName,
						Namespace: "default",
					},
				})
				Expect(err).NotTo(HaveOccurred())
			},
			Entry("OpenShift cluster type", constants.ClusterTypeOpenShift, "openshift-tuning", "test-kruize-openshift"),
			Entry("minikube cluster type", constants.ClusterTypeMinikube, "kruize", "test-kruize-minikube"),
			Entry("kind cluster type", constants.ClusterTypeKind, "kruize", "test-kruize-kind"),
		)

		DescribeTable("should reject invalid cluster types",
			func(clusterType, testName, expectedErrorSubstring string, shouldCheckSupportedTypes bool) {
				kruize := &kruizev1alpha1.Kruize{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testName,
						Namespace: "default",
					},
					Spec: kruizev1alpha1.KruizeSpec{
						Cluster_type:      clusterType,
						Namespace:         "test",
						Autotune_image:    constants.GetDefaultAutotuneImage(),
						Autotune_ui_image: constants.GetDefaultUIImage(),
					},
				}
				Expect(k8sClient.Create(ctx, kruize)).To(Succeed())

				defer func() {
					Expect(k8sClient.Delete(ctx, kruize)).To(Succeed())
				}()

				controllerReconciler := &KruizeReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testName,
						Namespace: "default",
					},
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported cluster type"))
				if expectedErrorSubstring != "" {
					Expect(err.Error()).To(ContainSubstring(expectedErrorSubstring))
				}
				if shouldCheckSupportedTypes {
					Expect(err.Error()).To(ContainSubstring("Supported types are:"))
				}
			},
			Entry("invalid cluster type", "invalid-cluster", "test-kruize-invalid", "invalid-cluster", false),
			Entry("empty cluster type", "", "test-kruize-empty", "", false),
			Entry("unknown cluster type with supported types message", "unknown", "test-kruize-unknown", "", true),
		)

		DescribeTable("should handle case-insensitive cluster types",
			func(clusterType, namespace, testName string) {
				kruize := &kruizev1alpha1.Kruize{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testName,
						Namespace: "default",
					},
					Spec: kruizev1alpha1.KruizeSpec{
						Cluster_type:      clusterType,
						Namespace:         namespace,
						Autotune_image:    constants.GetDefaultAutotuneImage(),
						Autotune_ui_image: constants.GetDefaultUIImage(),
					},
				}
				Expect(k8sClient.Create(ctx, kruize)).To(Succeed())

				defer func() {
					Expect(k8sClient.Delete(ctx, kruize)).To(Succeed())
				}()

				controllerReconciler := &KruizeReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testName,
						Namespace: "default",
					},
				})
				Expect(err).NotTo(HaveOccurred())
			},
			Entry("OpenShift with capital O", "OpenShift", "openshift-tuning", "test-kruize-openshift-capital"),
			Entry("MINIKUBE all caps", "MINIKUBE", "kruize", "test-kruize-minikube-caps"),
			Entry("Kind with capital K", "Kind", "kruize", "test-kruize-kind-capital"),
			Entry("MixedCase minikube", "MiniKube", "kruize", "test-kruize-minikube-mixed"),
		)

		DescribeTable("should not create resources for unsupported cluster types",
			func(clusterType, testName string) {
				testNamespace := "test-" + clusterType + "-namespace"
				kruize := &kruizev1alpha1.Kruize{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testName,
						Namespace: "default",
					},
					Spec: kruizev1alpha1.KruizeSpec{
						Cluster_type:      clusterType,
						Namespace:         testNamespace,
						Autotune_image:    constants.GetDefaultAutotuneImage(),
						Autotune_ui_image: constants.GetDefaultUIImage(),
					},
				}
				Expect(k8sClient.Create(ctx, kruize)).To(Succeed())

				defer func() {
					Expect(k8sClient.Delete(ctx, kruize)).To(Succeed())
				}()

				controllerReconciler := &KruizeReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				// Attempt reconciliation - should fail with validation error
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testName,
						Namespace: "default",
					},
				})

				// Verify error is returned
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported cluster type"))
				Expect(err.Error()).To(ContainSubstring(clusterType))

				// Verify no namespace was created for Kruize components
				namespaceList := &corev1.NamespaceList{}
				err = k8sClient.List(ctx, namespaceList)
				Expect(err).NotTo(HaveOccurred())

				namespaceExists := false
				for _, ns := range namespaceList.Items {
					if ns.Name == testNamespace {
						namespaceExists = true
						break
					}
				}
				Expect(namespaceExists).To(BeFalse(), "Namespace should not be created for invalid cluster type")

				// Verify no deployments were created in the test namespace
				deploymentList := &appsv1.DeploymentList{}
				err = k8sClient.List(ctx, deploymentList, client.InNamespace(testNamespace))
				if err == nil {
					Expect(deploymentList.Items).To(BeEmpty(), "No deployments should be created for invalid cluster type")
				}

				// Verify no services were created in the test namespace
				serviceList := &corev1.ServiceList{}
				err = k8sClient.List(ctx, serviceList, client.InNamespace(testNamespace))
				if err == nil {
					Expect(serviceList.Items).To(BeEmpty(), "No services should be created for invalid cluster type")
				}
			},
			Entry("gke cluster type", "gke", "test-kruize-gke"),
			Entry("eks cluster type", "eks", "test-kruize-eks"),
		)
	})

	Context("Resource generation", func() {
		It("should generate cluster-scoped resources for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())
			clusterResources := generator.ClusterScopedResources()
			Expect(clusterResources).NotTo(BeEmpty())
			Expect(len(clusterResources)).To(BeNumerically(">", 0))
		})

		It("should generate namespaced resources for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())
			namespacedResources := generator.NamespacedResources()
			Expect(namespacedResources).NotTo(BeEmpty())
			Expect(len(namespacedResources)).To(BeNumerically(">", 0))
		})

		It("should generate Kubernetes cluster-scoped resources", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeMinikube, &kruizev1alpha1.KruizeSpec{}, getTestContext())
			clusterResources := generator.KubernetesClusterScopedResources()
			Expect(clusterResources).NotTo(BeEmpty())
			Expect(len(clusterResources)).To(BeNumerically(">", 0))
		})

		It("should generate Kubernetes namespaced resources", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeMinikube, &kruizev1alpha1.KruizeSpec{}, getTestContext())
			namespacedResources := generator.KubernetesNamespacedResources()
			Expect(namespacedResources).NotTo(BeEmpty())
			Expect(len(namespacedResources)).To(BeNumerically(">", 0))
		})

		It("should use default images when not specified", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())
			Expect(generator.Autotune_image).To(Equal(constants.GetDefaultAutotuneImage()))
			Expect(generator.Autotune_ui_image).To(Equal(constants.GetDefaultUIImage()))
		})

		It("should use custom images when specified", func() {
			customImage := "custom/image:v1.0"
			customUIImage := "custom/ui:v1.0"
			customOptimizerImage := "custom/optimizer:v1.0"

			generator := utils.NewKruizeResourceGenerator("test-namespace", customImage, customUIImage, customOptimizerImage, constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			Expect(generator.Autotune_image).To(Equal(customImage))
			Expect(generator.Autotune_ui_image).To(Equal(customUIImage))
		})

		It("should apply custom ResourceConfig to Kruize and Database deployments", func() {
			cpuRequest := "200m"
			cpuLimit := "500m"
			memRequest := "256Mi"
			memLimit := "512Mi"

			kruize := &kruizev1alpha1.Kruize{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kruize",
					Namespace: "test-namespace",
				},
				Spec: kruizev1alpha1.KruizeSpec{
					KruizeDB: createKruizeDBConfig(cpuRequest, cpuLimit, memRequest, memLimit),
					Kruize:   createKruizeAppConfig(cpuRequest, cpuLimit, memRequest, memLimit),
				},
			}

			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruize.Spec, getTestContext())

			namespacedResources := generator.NamespacedResources()
			Expect(namespacedResources).NotTo(BeEmpty())

			kruizeDeployment := findDeployment(namespacedResources, "kruize")
			dbDeployment := findDeployment(namespacedResources, "kruize-db-deployment")

			Expect(kruizeDeployment).NotTo(BeNil(), "expected Kruize deployment to be generated")
			Expect(dbDeployment).NotTo(BeNil(), "expected Database deployment to be generated")

			// Verify Kruize container resources
			kruizeContainer := findContainerByName(kruizeDeployment.Spec.Template.Spec.Containers, "kruize")
			Expect(kruizeContainer).NotTo(BeNil(), "expected to find kruize container")
			Expect(kruizeContainer.Resources.Requests.Cpu().String()).To(Equal(cpuRequest))
			Expect(kruizeContainer.Resources.Limits.Cpu().String()).To(Equal(cpuLimit))
			Expect(kruizeContainer.Resources.Requests.Memory().String()).To(Equal(memRequest))
			Expect(kruizeContainer.Resources.Limits.Memory().String()).To(Equal(memLimit))

			// Verify Database container resources
			dbContainer := findContainerByName(dbDeployment.Spec.Template.Spec.Containers, "kruize-db")
			Expect(dbContainer).NotTo(BeNil(), "expected to find kruize-db container")
			Expect(dbContainer.Resources.Requests.Cpu().String()).To(Equal(cpuRequest))
			Expect(dbContainer.Resources.Limits.Cpu().String()).To(Equal(cpuLimit))
			Expect(dbContainer.Resources.Requests.Memory().String()).To(Equal(memRequest))
			Expect(dbContainer.Resources.Limits.Memory().String()).To(Equal(memLimit))
		})

		It("should allow partial resource overrides while preserving defaults", func() {
			// First, generate resources with default configuration to capture default resources
			defaultGenerator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())
			defaultNamespacedResources := defaultGenerator.NamespacedResources()

			defaultKruizeDeployment := findDeployment(defaultNamespacedResources, "kruize")
			defaultDBDeployment := findDeployment(defaultNamespacedResources, "kruize-db-deployment")

			Expect(defaultKruizeDeployment).NotTo(BeNil(), "default Kruize deployment should exist")
			Expect(defaultDBDeployment).NotTo(BeNil(), "default DB deployment should exist")

			defaultKruizeContainer := findContainerByName(defaultKruizeDeployment.Spec.Template.Spec.Containers, "kruize")
			defaultDBContainer := findContainerByName(defaultDBDeployment.Spec.Template.Spec.Containers, "kruize-db")
			Expect(defaultKruizeContainer).NotTo(BeNil(), "default kruize container should exist")
			Expect(defaultDBContainer).NotTo(BeNil(), "default kruize-db container should exist")

			// Now, create a spec that only overrides the CPU request for both components
			partialSpec := &kruizev1alpha1.KruizeSpec{
				KruizeDB: createKruizeDBConfig("250m", "", "", ""),
				Kruize:   createKruizeAppConfig("300m", "", "", ""),
			}
			partialGenerator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, partialSpec, getTestContext())
			partialNamespacedResources := partialGenerator.NamespacedResources()

			partialKruizeDeployment := findDeployment(partialNamespacedResources, "kruize")
			partialDBDeployment := findDeployment(partialNamespacedResources, "kruize-db-deployment")

			Expect(partialKruizeDeployment).NotTo(BeNil(), "partial Kruize deployment should exist")
			Expect(partialDBDeployment).NotTo(BeNil(), "partial DB deployment should exist")

			partialKruizeContainer := findContainerByName(partialKruizeDeployment.Spec.Template.Spec.Containers, "kruize")
			partialDBContainer := findContainerByName(partialDBDeployment.Spec.Template.Spec.Containers, "kruize-db")
			Expect(partialKruizeContainer).NotTo(BeNil(), "partial kruize container should exist")
			Expect(partialDBContainer).NotTo(BeNil(), "partial kruize-db container should exist")

			// Verify that the CPU request was overridden for DB
			Expect(partialDBContainer.Resources.Requests.Cpu().String()).To(Equal("250m"))
			// Verify that CPU limit and memory request/limit are still using the defaults for DB
			Expect(partialDBContainer.Resources.Limits.Cpu().String()).To(Equal(defaultDBContainer.Resources.Limits.Cpu().String()))
			Expect(partialDBContainer.Resources.Requests.Memory().String()).To(Equal(defaultDBContainer.Resources.Requests.Memory().String()))
			Expect(partialDBContainer.Resources.Limits.Memory().String()).To(Equal(defaultDBContainer.Resources.Limits.Memory().String()))

			// Verify that the CPU request was overridden for Kruize
			Expect(partialKruizeContainer.Resources.Requests.Cpu().String()).To(Equal("300m"))
			// Verify that CPU limit and memory request/limit are still using the defaults for Kruize
			Expect(partialKruizeContainer.Resources.Limits.Cpu().String()).To(Equal(defaultKruizeContainer.Resources.Limits.Cpu().String()))
			Expect(partialKruizeContainer.Resources.Requests.Memory().String()).To(Equal(defaultKruizeContainer.Resources.Requests.Memory().String()))
			Expect(partialKruizeContainer.Resources.Limits.Memory().String()).To(Equal(defaultKruizeContainer.Resources.Limits.Memory().String()))
		})
	})

	Context("RBAC and ConfigMap manifest generation", func() {
		It("should generate RBAC manifests correctly for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			clusterResources := generator.ClusterScopedResources()

			// Check that RBAC resources are present
			var hasClusterRole, hasClusterRoleBinding bool
			for _, resource := range clusterResources {
				kind := resource.GetObjectKind().GroupVersionKind().Kind
				if kind == "ClusterRole" {
					hasClusterRole = true
				}
				if kind == "ClusterRoleBinding" {
					hasClusterRoleBinding = true
				}
			}

			Expect(hasClusterRole).To(BeTrue(), "ClusterRole should be generated")
			Expect(hasClusterRoleBinding).To(BeTrue(), "ClusterRoleBinding should be generated")
		})

		It("should generate RBAC manifests correctly for Kubernetes", func() {

			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeMinikube, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			clusterResources := generator.KubernetesClusterScopedResources()

			// Check that RBAC resources are present
			var hasClusterRole, hasClusterRoleBinding bool
			for _, resource := range clusterResources {
				kind := resource.GetObjectKind().GroupVersionKind().Kind
				if kind == "ClusterRole" {
					hasClusterRole = true
				}
				if kind == "ClusterRoleBinding" {
					hasClusterRoleBinding = true
				}
			}

			Expect(hasClusterRole).To(BeTrue(), "ClusterRole should be generated")
			Expect(hasClusterRoleBinding).To(BeTrue(), "ClusterRoleBinding should be generated")
		})

		It("should generate ConfigMap correctly for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			configMap := generator.KruizeConfigMap()
			Expect(configMap).NotTo(BeNil())
			Expect(configMap.GetName()).To(Equal("kruizeconfig"))
			Expect(configMap.GetNamespace()).To(Equal("test-namespace"))
			Expect(configMap.Data).NotTo(BeEmpty())
		})

		It("should generate ConfigMap correctly for Kubernetes", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeMinikube, &kruizev1alpha1.KruizeSpec{}, getTestContext())
			configMap := generator.KruizeConfigMapKubernetes()
			Expect(configMap).NotTo(BeNil())
			Expect(configMap.GetName()).To(Equal("kruizeconfig"))
			Expect(configMap.GetNamespace()).To(Equal("test-namespace"))
			Expect(configMap.Data).NotTo(BeEmpty())
		})
	})

	Context("Data source configuration validation", func() {
		It("should have valid data source configuration in ConfigMap for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			configMap := generator.KruizeConfigMap()
			Expect(configMap.Data).To(HaveKey("kruizeconfigjson"))

			// Verify the config contains expected data source fields
			configData := configMap.Data["kruizeconfigjson"]
			Expect(configData).To(ContainSubstring("datasource"))
		})

		It("should have valid data source configuration in ConfigMap for Kubernetes", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeMinikube, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			configMap := generator.KruizeConfigMapKubernetes()
			Expect(configMap.Data).To(HaveKey("kruizeconfigjson"))

			// Verify the config contains expected data source fields
			configData := configMap.Data["kruizeconfigjson"]
			Expect(configData).To(ContainSubstring("datasource"))
		})
	})

	Context("Kruize deployment manifest generation", func() {
		DescribeTable("should generate valid Kruize deployment manifest with default resources",
			func(clusterType string, resourceMethod func(*utils.KruizeResourceGenerator) []client.Object) {
				generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", clusterType, &kruizev1alpha1.KruizeSpec{}, getTestContext())

				namespacedResources := resourceMethod(generator)

				// Check for Deployment resources and validate default resource configuration
				kruizeDeployment := findDeployment(namespacedResources, "kruize")
				kruizeDBDeployment := findDeployment(namespacedResources, "kruize-db-deployment")

				Expect(kruizeDeployment).NotTo(BeNil(), "Kruize deployment should be generated")
				Expect(kruizeDBDeployment).NotTo(BeNil(), "Kruize DB deployment should be generated")

				// Validate Kruize deployment has default resource configuration
				kruizeContainer := getContainer(kruizeDeployment, "kruize")
				Expect(kruizeContainer.Resources.Requests.Cpu().String()).To(Equal("700m"))
				Expect(kruizeContainer.Resources.Requests.Memory().String()).To(Equal("768Mi"))
				Expect(kruizeContainer.Resources.Limits.Cpu().String()).To(Equal("700m"))
				Expect(kruizeContainer.Resources.Limits.Memory().String()).To(Equal("768Mi"))

				// Validate DB deployment has default resource configuration
				dbContainer := getContainer(kruizeDBDeployment, "kruize-db")
				Expect(dbContainer.Resources.Requests.Cpu().String()).To(Equal("500m"))
				Expect(dbContainer.Resources.Requests.Memory().String()).To(Equal("100Mi"))
				Expect(dbContainer.Resources.Limits.Cpu().String()).To(Equal("500m"))
				Expect(dbContainer.Resources.Limits.Memory().String()).To(Equal("100Mi"))
			},
			Entry("for OpenShift", constants.ClusterTypeOpenShift, func(g *utils.KruizeResourceGenerator) []client.Object {
				return g.NamespacedResources()
			}),
			Entry("for Kubernetes", constants.ClusterTypeMinikube, func(g *utils.KruizeResourceGenerator) []client.Object {
				return g.KubernetesNamespacedResources()
			}),
		)

		DescribeTable("should generate valid Kruize deployment manifest with custom resources",
			func(clusterType string, resourceMethod func(*utils.KruizeResourceGenerator) []client.Object) {
				customSpec := &kruizev1alpha1.KruizeSpec{
					Kruize:   createKruizeAppConfig("2.0", "2.5", "1Gi", "1.5Gi"),
					KruizeDB: createKruizeDBConfig("0.75", "1.5", "512Mi", "1Gi"),
				}
				generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", clusterType, customSpec, getTestContext())
				namespacedResources := resourceMethod(generator)

				// Check for Deployment resources and validate custom resource configuration
				kruizeDeployment := findDeployment(namespacedResources, "kruize")
				kruizeDBDeployment := findDeployment(namespacedResources, "kruize-db-deployment")

				Expect(kruizeDeployment).NotTo(BeNil(), "Kruize deployment should be generated")
				Expect(kruizeDBDeployment).NotTo(BeNil(), "Kruize DB deployment should be generated")

				// Validate Kruize deployment has custom resource configuration
				kruizeContainer := getContainer(kruizeDeployment, "kruize")
				Expect(kruizeContainer.Resources.Requests.Cpu().String()).To(Equal("2"))
				Expect(kruizeContainer.Resources.Requests.Memory().String()).To(Equal("1Gi"))
				Expect(kruizeContainer.Resources.Limits.Cpu().String()).To(Equal("2500m"))
				Expect(kruizeContainer.Resources.Limits.Memory().String()).To(Equal("1536Mi"))

				// Validate DB deployment has custom resource configuration
				dbContainer := getContainer(kruizeDBDeployment, "kruize-db")
				Expect(dbContainer.Resources.Requests.Cpu().String()).To(Equal("750m"))
				Expect(dbContainer.Resources.Requests.Memory().String()).To(Equal("512Mi"))
				Expect(dbContainer.Resources.Limits.Cpu().String()).To(Equal("1500m"))
				Expect(dbContainer.Resources.Limits.Memory().String()).To(Equal("1Gi"))
			},
			Entry("for OpenShift", constants.ClusterTypeOpenShift, func(g *utils.KruizeResourceGenerator) []client.Object {
				return g.NamespacedResources()
			}),
			Entry("for Kubernetes", constants.ClusterTypeMinikube, func(g *utils.KruizeResourceGenerator) []client.Object {
				return g.KubernetesNamespacedResources()
			}),
		)
	})

	Context("Pod creation validation", func() {
		It("should generate Kruize pod specification", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize deployment
			kruizeDeployment := findDeployment(namespacedResources, "kruize")
			getContainer(kruizeDeployment, "kruize")
		})

		It("should generate Kruize-ui deployment specification", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			namespacedResources := generator.NamespacedResources()
			
			// Find the Kruize UI deployment
			var kruizeUIDeployment *appsv1.Deployment
			for _, resource := range namespacedResources {
				if resource.GetObjectKind().GroupVersionKind().Kind == "Deployment" && resource.GetName() == "kruize-ui-nginx" {
					var ok bool
					kruizeUIDeployment, ok = resource.(*appsv1.Deployment)
					Expect(ok).To(BeTrue(), "Resource should be a valid Deployment")
					break
				}
			}
			
			Expect(kruizeUIDeployment).NotTo(BeNil(), "Kruize UI deployment should exist")
			Expect(kruizeUIDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
		})

		It("should generate Kruize-db pod specification", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize DB deployment
			kruizeDBDeployment := findDeployment(namespacedResources, "kruize-db-deployment")
			getContainer(kruizeDBDeployment, "kruize-db")
		})

		It("should apply custom resource configuration to Kruize deployment", func() {
			customSpec := &kruizev1alpha1.KruizeSpec{
				Kruize: createKruizeAppConfig("1.0", "2.0", "1Gi", "2Gi"),
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, customSpec, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize deployment
			kruizeDeployment := findDeployment(namespacedResources, "kruize")
			container := getContainer(kruizeDeployment, "kruize")
			
			// Verify custom resource requests
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("1"))
			Expect(container.Resources.Requests.Memory().String()).To(Equal("1Gi"))
			
			// Verify custom resource limits
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("2"))
			Expect(container.Resources.Limits.Memory().String()).To(Equal("2Gi"))
		})

		It("should apply custom resource configuration to Database deployment", func() {
			customSpec := &kruizev1alpha1.KruizeSpec{
				KruizeDB: createKruizeDBConfig("0.25", "1.0", "256Mi", "512Mi"),
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, customSpec, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize DB deployment
			kruizeDBDeployment := findDeployment(namespacedResources, "kruize-db-deployment")
			container := getContainer(kruizeDBDeployment, "kruize-db")
			
			// Verify custom resource requests
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("250m"))
			Expect(container.Resources.Requests.Memory().String()).To(Equal("256Mi"))
			
			// Verify custom resource limits
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("1"))
			Expect(container.Resources.Limits.Memory().String()).To(Equal("512Mi"))
		})

		It("should use default resources when ResourceConfig is nil", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "",constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize deployment
			kruizeDeployment := findDeployment(namespacedResources, "kruize")
			container := getContainer(kruizeDeployment, "kruize")
			
			// Verify default resource requests
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("700m"))
			Expect(container.Resources.Requests.Memory().String()).To(Equal("768Mi"))
			
			// Verify default resource limits
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("700m"))
			Expect(container.Resources.Limits.Memory().String()).To(Equal("768Mi"))
		})

		It("should apply partial custom resource configuration with defaults", func() {
			customSpec := &kruizev1alpha1.KruizeSpec{
				Kruize: createKruizeAppConfig("1.5", "", "", "3Gi"),
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, customSpec, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize deployment
			kruizeDeployment := findDeployment(namespacedResources, "kruize")
			container := getContainer(kruizeDeployment, "kruize")
			
			// Verify custom CPU request
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("1500m"))
			// Verify default memory request
			Expect(container.Resources.Requests.Memory().String()).To(Equal("768Mi"))
			
			// Verify default CPU limit
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("700m"))
			// Verify custom memory limit
			Expect(container.Resources.Limits.Memory().String()).To(Equal("3Gi"))
		})
	})

	Context("Partial ResourceConfig edge cases", func() {
		It("should apply partial Database ResourceConfig with only CPURequest set", func() {
			customSpec := &kruizev1alpha1.KruizeSpec{
				KruizeDB: createKruizeDBConfig("0.3", "", "", ""),
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, customSpec, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize DB deployment
			kruizeDBDeployment := findDeployment(namespacedResources, "kruize-db-deployment")
			container := getContainer(kruizeDBDeployment, "kruize-db")
			
			// Verify custom CPU request
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("300m"))
			// Verify default values for unset fields
			Expect(container.Resources.Requests.Memory().String()).To(Equal("100Mi"))
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("500m"))
			Expect(container.Resources.Limits.Memory().String()).To(Equal("100Mi"))
		})

		It("should apply partial Kruize ResourceConfig with only MemoryLimit set", func() {
			customSpec := &kruizev1alpha1.KruizeSpec{
				Kruize: createKruizeAppConfig("", "", "", "2Gi"),
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeMinikube, customSpec, getTestContext())

			namespacedResources := generator.KubernetesNamespacedResources()

			// Find the Kruize deployment
			kruizeDeployment := findDeployment(namespacedResources, "kruize")
			container := getContainer(kruizeDeployment, "kruize")
			
			// Verify default values for unset fields
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("700m"))
			Expect(container.Resources.Requests.Memory().String()).To(Equal("768Mi"))
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("700m"))
			// Verify custom memory limit
			Expect(container.Resources.Limits.Memory().String()).To(Equal("2Gi"))
		})

		It("should apply partial Database ResourceConfig with mixed custom and default values", func() {
			customSpec := &kruizev1alpha1.KruizeSpec{
				KruizeDB: createKruizeDBConfig("", "1.0", "256Mi", ""),
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeMinikube, customSpec, getTestContext())

			namespacedResources := generator.KubernetesNamespacedResources()

			// Find the Kruize DB deployment
			kruizeDBDeployment := findDeployment(namespacedResources, "kruize-db-deployment")
			container := getContainer(kruizeDBDeployment, "kruize-db")
			
			// Verify default CPU request
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("500m"))
			// Verify custom memory request
			Expect(container.Resources.Requests.Memory().String()).To(Equal("256Mi"))
			// Verify custom CPU limit
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("1"))
			// Verify default memory limit
			Expect(container.Resources.Limits.Memory().String()).To(Equal("100Mi"))
		})
	})

	Context("PersistentVolume and PVC configuration", func() {
		It("should apply custom PV/PVC configuration for OpenShift", func() {
			customSpec := &kruizev1alpha1.KruizeSpec{
				PersistentVolume:      createPVSpec("2Gi", "custom-storage", "/custom/path", []kruizev1alpha1.PersistentVolumeAccessMode{kruizev1alpha1.ReadWriteOnce}),
				PersistentVolumeClaim: createPVCSpec("1Gi"),
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, customSpec, getTestContext())

			clusterResources := generator.ClusterScopedResources()

			// Find the PV
			pv := findPersistentVolume(clusterResources)
			Expect(pv).NotTo(BeNil(), "PersistentVolume should exist")
			Expect(pv.Spec.Capacity.Storage().String()).To(Equal("2Gi"))
			Expect(pv.Spec.StorageClassName).To(Equal("custom-storage"))
			Expect(pv.Spec.PersistentVolumeSource.HostPath.Path).To(Equal("/custom/path"))
			Expect(pv.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}))

			// Find the PVC
			pvc := findPersistentVolumeClaim(clusterResources)
			Expect(pvc).NotTo(BeNil(), "PersistentVolumeClaim should exist")
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("1Gi"))
			Expect(*pvc.Spec.StorageClassName).To(Equal("custom-storage"))
			Expect(pvc.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}))
		})

		It("should apply custom PV/PVC configuration for Kubernetes", func() {
			customSpec := &kruizev1alpha1.KruizeSpec{
				PersistentVolume:      createPVSpec("3Gi", "k8s-storage", "/k8s/custom/path", []kruizev1alpha1.PersistentVolumeAccessMode{kruizev1alpha1.ReadWriteMany}),
				PersistentVolumeClaim: createPVCSpec("2Gi"),
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeMinikube, customSpec, getTestContext())

			clusterResources := generator.KubernetesClusterScopedResources()

			// Find the PV
			pv := findPersistentVolume(clusterResources)
			Expect(pv).NotTo(BeNil(), "PersistentVolume should exist")
			Expect(pv.Spec.Capacity.Storage().String()).To(Equal("3Gi"))
			Expect(pv.Spec.StorageClassName).To(Equal("k8s-storage"))
			Expect(pv.Spec.PersistentVolumeSource.HostPath.Path).To(Equal("/k8s/custom/path"))
			Expect(pv.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}))

			// Find the PVC
			pvc := findPersistentVolumeClaim(clusterResources)
			Expect(pvc).NotTo(BeNil(), "PersistentVolumeClaim should exist")
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("2Gi"))
			Expect(*pvc.Spec.StorageClassName).To(Equal("k8s-storage"))
			Expect(pvc.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}))
		})

		It("should fallback PVCStorageSize to PVStorageSize when PVCStorageSize is omitted for OpenShift", func() {
			customSpec := &kruizev1alpha1.KruizeSpec{
				PersistentVolume: createPVSpec("5Gi", "fallback-storage", "/fallback/path", nil),
				// PVC not specified, should use PV storage size
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, customSpec, getTestContext())

			clusterResources := generator.ClusterScopedResources()

			// Find the PV
			pv := findPersistentVolume(clusterResources)
			Expect(pv).NotTo(BeNil(), "PersistentVolume should exist")
			Expect(pv.Spec.Capacity.Storage().String()).To(Equal("5Gi"))

			// Find the PVC and verify it uses PVStorageSize
			pvc := findPersistentVolumeClaim(clusterResources)
			Expect(pvc).NotTo(BeNil(), "PersistentVolumeClaim should exist")
			// PVC should use PVStorageSize since PVCStorageSize was not specified
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("5Gi"))
		})

		It("should fallback PVCStorageSize to PVStorageSize when PVCStorageSize is omitted for Kubernetes", func() {
			customSpec := &kruizev1alpha1.KruizeSpec{
				PersistentVolume: createPVSpec("4Gi", "k8s-fallback", "/k8s/fallback", nil),
				// PVC not specified, should use PV storage size
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeMinikube, customSpec, getTestContext())

			clusterResources := generator.KubernetesClusterScopedResources()

			// Find the PV
			pv := findPersistentVolume(clusterResources)
			Expect(pv).NotTo(BeNil(), "PersistentVolume should exist")
			Expect(pv.Spec.Capacity.Storage().String()).To(Equal("4Gi"))

			// Find the PVC and verify it uses PVStorageSize
			pvc := findPersistentVolumeClaim(clusterResources)
			Expect(pvc).NotTo(BeNil(), "PersistentVolumeClaim should exist")
			// PVC should use PVStorageSize since PVCStorageSize was not specified
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("4Gi"))
		})

		It("should use default PV/PVC configuration when ResourceConfig is nil for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			clusterResources := generator.ClusterScopedResources()

			// Find the PV
			pv := findPersistentVolume(clusterResources)
			Expect(pv).NotTo(BeNil(), "PersistentVolume should exist")
			Expect(pv.Spec.Capacity.Storage().String()).To(Equal("500Mi"))
			Expect(pv.Spec.StorageClassName).To(Equal("manual"))
			Expect(pv.Spec.PersistentVolumeSource.HostPath.Path).To(Equal("/mnt/data"))
			Expect(pv.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}))

			// Find the PVC
			pvc := findPersistentVolumeClaim(clusterResources)
			Expect(pvc).NotTo(BeNil(), "PersistentVolumeClaim should exist")
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("500Mi"))
			Expect(*pvc.Spec.StorageClassName).To(Equal("manual"))
			Expect(pvc.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}))
		})

		It("should use default PV/PVC configuration when ResourceConfig is nil for Kubernetes", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeMinikube, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			clusterResources := generator.KubernetesClusterScopedResources()

			// Find the PV
			pv := findPersistentVolume(clusterResources)
			Expect(pv).NotTo(BeNil(), "PersistentVolume should exist")
			Expect(pv.Spec.Capacity.Storage().String()).To(Equal("1Gi"))
			Expect(pv.Spec.StorageClassName).To(Equal("manual"))
			Expect(pv.Spec.PersistentVolumeSource.HostPath.Path).To(Equal("/tmp/data"))
			Expect(pv.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}))

			// Find the PVC
			pvc := findPersistentVolumeClaim(clusterResources)
			Expect(pvc).NotTo(BeNil(), "PersistentVolumeClaim should exist")
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("1Gi"))
			Expect(*pvc.Spec.StorageClassName).To(Equal("manual"))
			Expect(pvc.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}))
		})
	})

	Context("Route and service creation", func() {
		It("should generate routes for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())
			namespacedResources := generator.NamespacedResources()

			// Check for Route resources
			var hasKruizeRoute, hasUIRoute bool
			for _, resource := range namespacedResources {
				kind := resource.GetObjectKind().GroupVersionKind().Kind
				name := resource.GetName()

				if kind == "Route" && name == "kruize" {
					hasKruizeRoute = true
				}
				if kind == "Route" && name == "kruize-ui-nginx-service" {
					hasUIRoute = true
				}
			}

			Expect(hasKruizeRoute).To(BeTrue(), "Kruize route should be generated")
			Expect(hasUIRoute).To(BeTrue(), "Kruize UI route should be generated")
		})

		It("should generate services for all cluster types", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Check for Service resources
			var hasKruizeService, hasDBService, hasUIService bool
			for _, resource := range namespacedResources {
				kind := resource.GetObjectKind().GroupVersionKind().Kind
				name := resource.GetName()

				if kind == "Service" && name == "kruize" {
					hasKruizeService = true
				}
				if kind == "Service" && name == "kruize-db-service" {
					hasDBService = true
				}
				if kind == "Service" && name == "kruize-ui-nginx-service" {
					hasUIService = true
				}
			}

			Expect(hasKruizeService).To(BeTrue(), "Kruize service should be generated")
			Expect(hasDBService).To(BeTrue(), "Kruize DB service should be generated")
			Expect(hasUIService).To(BeTrue(), "Kruize UI service should be generated")
		})
	})

	Context("Kruize endpoints validation", func() {
		It("should generate service with correct ports for Kruize", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize service
			kruizeService := findTypedResource[*corev1.Service](namespacedResources, "kruize", "", "")
			Expect(kruizeService).NotTo(BeNil(), "Kruize service should exist")
			Expect(kruizeService.Spec.Ports).NotTo(BeEmpty(), "Service should have ports defined")

			// Verify the service has the expected port
			var hasKruizePort bool
			for _, port := range kruizeService.Spec.Ports {
				if port.Name == "kruize-port" {
					hasKruizePort = true
					Expect(port.Port).To(Equal(int32(8080)))
				}
			}
			Expect(hasKruizePort).To(BeTrue(), "Service should have kruize-port defined")
		})

		It("should generate service with correct ports for Kruize UI", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())


			namespacedResources := generator.NamespacedResources()

			// Find the Kruize UI service
			kruizeUIService := findTypedResource[*corev1.Service](namespacedResources, "kruize-ui-nginx-service", "", "")
			Expect(kruizeUIService).NotTo(BeNil(), "Kruize UI service should exist")
			Expect(kruizeUIService.Spec.Ports).NotTo(BeEmpty(), "Service should have ports defined")

			// Verify the service has the expected port
			var hasUIPort bool
			for _, port := range kruizeUIService.Spec.Ports {
				if port.Name == "http" {
					hasUIPort = true
					Expect(port.Port).To(Equal(int32(8080)))
				}
			}
			Expect(hasUIPort).To(BeTrue(), "Service should have kruize-ui http port defined")
		})

		It("should generate service with correct ports for Kruize DB", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())


			namespacedResources := generator.NamespacedResources()

			// Find the Kruize DB service
			kruizeDBService := findTypedResource[*corev1.Service](namespacedResources, "kruize-db-service", "", "")
			Expect(kruizeDBService).NotTo(BeNil(), "Kruize DB service should exist")
			Expect(kruizeDBService.Spec.Ports).NotTo(BeEmpty(), "Service should have ports defined")

			// Verify the service has the expected port
			var hasDBPort bool
			for _, port := range kruizeDBService.Spec.Ports {
				if port.Name == "kruize-db-port" {
					hasDBPort = true
					Expect(port.Port).To(Equal(int32(5432)))
				}
			}
			Expect(hasDBPort).To(BeTrue(), "Service should have kruize-db-port defined")
		})

		It("should generate Kruize service with NodePort type", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize service
			kruizeService := findTypedResource[*corev1.Service](namespacedResources, "kruize", "", "")
			Expect(kruizeService).NotTo(BeNil(), "Kruize service should exist")
			Expect(kruizeService.Spec.Type).To(Equal(corev1.ServiceTypeNodePort), "Kruize service should be NodePort type")
		})

		It("should generate Kruize UI service with NodePort type", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize UI service
			kruizeUIService := findTypedResource[*corev1.Service](namespacedResources, "kruize-ui-nginx-service", "", "")
			Expect(kruizeUIService).NotTo(BeNil(), "Kruize UI service should exist")
			Expect(kruizeUIService.Spec.Type).To(Equal(corev1.ServiceTypeNodePort), "Kruize UI service should be NodePort type")
		})

		It("should generate Kruize DB service with ClusterIP type", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", "", constants.ClusterTypeOpenShift, &kruizev1alpha1.KruizeSpec{}, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize DB service
			kruizeDBService := findTypedResource[*corev1.Service](namespacedResources, "kruize-db-service", "", "")
			Expect(kruizeDBService).NotTo(BeNil(), "Kruize DB service should exist")
			Expect(kruizeDBService.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP), "Kruize DB service should be ClusterIP type")
		})
	})

    Context("Image defaulting behavior", func() {
    	It("should use default images when not specified", func() {
    		namespace := "test-default-images"
    		clusterType := constants.ClusterTypeMinikube

    		By("creating a generator with empty image fields")
    		generator := utils.NewKruizeResourceGenerator(namespace, "", "", "", clusterType, &kruizev1alpha1.KruizeSpec{}, getTestContext())


    		By("verifying the generator uses the default-image helpers")
    		// This test verifies that the generator is wired to use the default-image helpers
    		Expect(generator.Autotune_image).To(Equal(constants.GetDefaultAutotuneImage()),
    			"Generator should default Autotune_image when empty")
    		Expect(generator.Autotune_ui_image).To(Equal(constants.GetDefaultUIImage()),
    			"Generator should default Autotune_ui_image when empty")

    		By("verifying the generated resources use default images")
    		namespacedResources := generator.KubernetesNamespacedResources()

    		// Find and verify Kruize deployment using helper
    		kruizeDeployment := findTypedResource[*appsv1.Deployment](namespacedResources, "kruize", "app", "kruize")
    		Expect(kruizeDeployment).NotTo(BeNil(), "Kruize deployment should be generated")
    		Expect(kruizeDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
    		
    		// Find the kruize (autotune) container by name using helper
    		kruizeContainer := findContainerByName(kruizeDeployment.Spec.Template.Spec.Containers, "kruize")
    		Expect(kruizeContainer).NotTo(BeNil(), "Kruize container should exist in deployment")
    		Expect(kruizeContainer.Image).To(Equal(constants.GetDefaultAutotuneImage()),
    			"Kruize deployment should use default Autotune image")
    
    		// Find and verify UI deployment using helper
    		kruizeUIDeployment := findTypedResource[*appsv1.Deployment](namespacedResources, "kruize-ui-nginx", "app", "kruize-ui-nginx")
    		Expect(kruizeUIDeployment).NotTo(BeNil(), "Kruize UI deployment should be generated")
    		Expect(kruizeUIDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
    		
    		// Find the kruize-ui-nginx container by name using helper
    		uiContainer := findContainerByName(kruizeUIDeployment.Spec.Template.Spec.Containers, "kruize-ui-nginx-container")
    		Expect(uiContainer).NotTo(BeNil(), "Kruize UI container should exist in deployment")
    		Expect(uiContainer.Image).To(Equal(constants.GetDefaultUIImage()),
    			"Kruize UI deployment should use default UI image")
    	})

    	It("should use custom images when specified", func() {
    		namespace := "test-custom-images"
    		clusterType := constants.ClusterTypeMinikube
    		customAutotuneImage := "custom.registry/autotune:custom-tag"
    		customUIImage := "custom.registry/ui:custom-tag"
			customOptimizerImage := "custom.registry/optimizer:custom-tag"

    		By("creating a generator with custom image values")
    		generator := utils.NewKruizeResourceGenerator(namespace, customAutotuneImage, customUIImage, customOptimizerImage, clusterType, &kruizev1alpha1.KruizeSpec{}, getTestContext())


    		By("verifying the generator uses the provided custom images")
    		Expect(generator.Autotune_image).To(Equal(customAutotuneImage),
    			"Generator should use provided Autotune_image")
    		Expect(generator.Autotune_ui_image).To(Equal(customUIImage),
    			"Generator should use provided Autotune_ui_image")
    	})
    })

    var _ = Describe("Metrics Auth Integration", func() {
        var client *http.Client

        BeforeEach(func() {
            // Create a client that ignores self-signed certs (standard for envtest)
            tr := &http.Transport{
                TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
            }
            client = &http.Client{Transport: tr}
        })

        Context("when accessing the metrics endpoint", func() {
            It("should reject unauthorized requests", func() {
                By("calling the metrics endpoint until it is ready")

                testMode := os.Getenv("KRUIZE_TEST_MODE")

                if testMode == "true" {
                    Eventually(func() (int, error) {
                        metricsURL := fmt.Sprintf("https://%s/metrics", utils.LocalMetricsAddr)
                        resp, err := client.Get(metricsURL)
                        if err != nil {
                            return 0, err // Return error to trigger a retry
                        }
                        defer resp.Body.Close()
                        return resp.StatusCode, nil
                    }, "10s", "1s").Should(SatisfyAny(
                        Equal(http.StatusUnauthorized),
                        Equal(http.StatusForbidden),
                        Equal(http.StatusInternalServerError),
                    ), "The metrics server should eventually be reachable and reject the unauthorized request in envtest")
                } else {
                    // In real clusters, 500 usually indicates misconfiguration, so do not accept it
                    Eventually(func() (int, error) {
                        metricsURL := fmt.Sprintf("https://%s/metrics", utils.LocalMetricsAddr)
                        resp, err := client.Get(metricsURL)
                        if err != nil {
                            return 0, err // Return error to trigger a retry
                        }
                        defer resp.Body.Close()
                        return resp.StatusCode, nil
                    }, "10s", "1s").Should(SatisfyAny(
                        Equal(http.StatusUnauthorized),
                        Equal(http.StatusForbidden),
                    ), "The metrics server should eventually be reachable and reject the unauthorized request in real-cluster mode")
                }
            })

            It("should fail when an invalid token is provided", func() {
                By("waiting for the metrics server to be ready")
                testMode := os.Getenv("KRUIZE_TEST_MODE")

                // Wait for metrics server readiness before testing invalid-token behavior
                if testMode == "true" {
                    Eventually(func() (int, error) {
                        metricsURL := fmt.Sprintf("https://%s/metrics", utils.LocalMetricsAddr)
                        resp, err := client.Get(metricsURL)
                        if err != nil {
                            return 0, err
                        }
                        defer resp.Body.Close()
                        return resp.StatusCode, nil
                    }, "10s", "1s").Should(SatisfyAny(
                        Equal(http.StatusUnauthorized),
                        Equal(http.StatusForbidden),
                        Equal(http.StatusInternalServerError),
                    ), "The metrics server should be reachable before testing invalid-token behavior")
                } else {
                    Eventually(func() (int, error) {
                        metricsURL := fmt.Sprintf("https://%s/metrics", utils.LocalMetricsAddr)
                        resp, err := client.Get(metricsURL)
                        if err != nil {
                            return 0, err
                        }
                        defer resp.Body.Close()
                        return resp.StatusCode, nil
                    }, "10s", "1s").Should(SatisfyAny(
                        Equal(http.StatusUnauthorized),
                        Equal(http.StatusForbidden),
                    ), "The metrics server should be reachable before testing invalid-token behavior")
                }

                By("executing the request with an invalid bearer token and readiness handling")
                metricsURL := fmt.Sprintf("https://%s/metrics", utils.LocalMetricsAddr)
                var resp *http.Response
                Eventually(func() error {
                    req, err := http.NewRequest("GET", metricsURL, nil)
                    if err != nil {
                        return err
                    }
                    req.Header.Set("Authorization", "Bearer invalid-token")
                    
                    resp, err = client.Do(req)
                    if err != nil {
                        return err // Retry on connection errors
                    }
                    return nil
                }, "10s", "1s").Should(Succeed(), "Should eventually connect to metrics server with invalid token")
                defer resp.Body.Close()

                By("verifying the metrics server intercepted the request")
                if os.Getenv("KRUIZE_TEST_MODE") == "true" {
                    // In envtest mode we explicitly expect the known 500 behavior,
                    // rather than treating any 500 as success.
                    Expect(resp.StatusCode).To(
                        Equal(http.StatusInternalServerError),
                        "Expected 500 from envtest when handling unauthenticated request",
                    )
                } else {
                    Expect(resp.StatusCode).To(SatisfyAny(
                        Equal(http.StatusUnauthorized),
                        Equal(http.StatusForbidden),
                    ), "Expected 401 or 403 for unauthenticated request")
                }
            })

            It("should have the metrics server running and reachable", func() {
                checkMetricsReachable := func() error {
                    metricsURL := fmt.Sprintf("https://%s/metrics", utils.LocalMetricsAddr)
                    resp, err := client.Get(metricsURL)
                    if err != nil {
                        return err
                    }
                    defer resp.Body.Close()

                    // Reachability test: getting any HTTP response proves the server is listening
                    if resp == nil {
                        return fmt.Errorf("expected non-nil response from metrics endpoint")
                    }
                    if resp.StatusCode == 0 {
                        return fmt.Errorf("expected non-zero HTTP status code from metrics endpoint")
                    }

                    return nil
                }

                By("waiting for the metrics server to become reachable on the secure port")
                Eventually(checkMetricsReachable).Should(Succeed())

                By("verifying the metrics server remains reachable on the secure port")
                Consistently(checkMetricsReachable).Should(Succeed())
            })
        })
    })
})
