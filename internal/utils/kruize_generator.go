package utils

import (
	"context"
	"fmt"
	kruizev1alpha1 "github.com/kruize/kruize-operator/api/v1alpha1"
	"github.com/kruize/kruize-operator/internal/constants"
	routev1 "github.com/openshift/api/route/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}

// KruizeResourceGenerator holds common data needed for creating resources.
type KruizeResourceGenerator struct {
	Namespace         string
	Autotune_image    string
	Autotune_ui_image string
	Optimizer_image   string
	ClusterType       string // "openshift", "minikube", or "kind"
	KruizeSpec        *kruizev1alpha1.KruizeSpec
	Ctx               context.Context
}

// NewKruizeResourceGenerator creates a new generator for Kruize resources.
func NewKruizeResourceGenerator(namespace string, autotuneImage string, autotuneUIImage string,  optimizerImage string, clusterType string, kruizeSpec *kruizev1alpha1.KruizeSpec, ctx context.Context) *KruizeResourceGenerator {
	// If no image is provided from the CR, use a sensible default.
	// The default can be configured via environment variables:
	// - DEFAULT_AUTOTUNE_IMAGE: Override the default Autotune image
	// - DEFAULT_AUTOTUNE_UI_IMAGE: Override the default Autotune UI image
	// - DEFAULT_OPTIMIZER_IMAGE: Override the default Optimizer image
	if autotuneImage == "" {
		autotuneImage = constants.GetDefaultAutotuneImage()
	}
	if autotuneUIImage == "" {
		autotuneUIImage = constants.GetDefaultUIImage()
	}
	if optimizerImage == "" {
		optimizerImage = constants.GetDefaultOptimizerImage()
	}
	if clusterType == "" {
		clusterType = constants.ClusterTypeOpenShift // Default to openshift for backward compatibility
	}
	return &KruizeResourceGenerator{
		Namespace:         namespace,
		Autotune_image:    autotuneImage,
		Autotune_ui_image: autotuneUIImage,
		Optimizer_image:   optimizerImage,
		ClusterType:       clusterType,
		KruizeSpec:        kruizeSpec,
		Ctx:               ctx,
	}
}

// getResourceValue returns the configured value or a default
func (g *KruizeResourceGenerator) getResourceValue(configValue, defaultValue string) string {
	if configValue != "" {
		return configValue
	}
	return defaultValue
}

// parseResourceQuantity safely parses a resource quantity string.
// If parsing the user-provided value fails, it falls back to the default value and logs a warning.
func (g *KruizeResourceGenerator) parseResourceQuantity(value, defaultValue string) resource.Quantity {
	// Parse the user-provided value first
	quantity, err := resource.ParseQuantity(value)
	if err == nil {
		return quantity
	}
	
	// If user value is invalid, log a warning and fall back to the default value
	logger := log.FromContext(g.Ctx)
	logger.Info("Invalid resource quantity specified, falling back to default",
		"provided", value,
		"default", defaultValue,
		"error", err.Error())
	
	// The default values are hardcoded constants, so MustParse is safe here
	return resource.MustParse(defaultValue)
}

// getDBResources returns database resource requirements with defaults
// For minikube/kind, defaults are disabled unless explicitly specified in the CR
func (g *KruizeResourceGenerator) getDBResources() corev1.ResourceRequirements {
	// For minikube/kind, only apply resources if explicitly specified in the CR
	if (g.ClusterType == constants.ClusterTypeMinikube || g.ClusterType == constants.ClusterTypeKind) {
		if g.KruizeSpec == nil || g.KruizeSpec.KruizeDB == nil || g.KruizeSpec.KruizeDB.Resources == nil {
			// Return empty resource requirements for minikube/kind when not specified
			return corev1.ResourceRequirements{}
		}
		// For minikube/kind with explicit resources, use only what's specified (no defaults)
		res := g.KruizeSpec.KruizeDB.Resources
		requirements := corev1.ResourceRequirements{}
		
		if res.Requests != nil {
			if res.Requests.CPU != "" || res.Requests.Memory != "" {
				requirements.Requests = corev1.ResourceList{}
				if res.Requests.CPU != "" {
					requirements.Requests[corev1.ResourceCPU] = resource.MustParse(res.Requests.CPU)
				}
				if res.Requests.Memory != "" {
					requirements.Requests[corev1.ResourceMemory] = resource.MustParse(res.Requests.Memory)
				}
			}
		}
		if res.Limits != nil {
			if res.Limits.CPU != "" || res.Limits.Memory != "" {
				requirements.Limits = corev1.ResourceList{}
				if res.Limits.CPU != "" {
					requirements.Limits[corev1.ResourceCPU] = resource.MustParse(res.Limits.CPU)
				}
				if res.Limits.Memory != "" {
					requirements.Limits[corev1.ResourceMemory] = resource.MustParse(res.Limits.Memory)
				}
			}
		}
		return requirements
	}

	// For OpenShift, use defaults
	cpuRequest := constants.DefaultDBCPURequest
	cpuLimit := constants.DefaultDBCPULimit
	memoryRequest := constants.DefaultDBMemoryRequest
	memoryLimit := constants.DefaultDBMemoryLimit

	if g.KruizeSpec != nil && g.KruizeSpec.KruizeDB != nil && g.KruizeSpec.KruizeDB.Resources != nil {
		res := g.KruizeSpec.KruizeDB.Resources
		if res.Requests != nil {
			cpuRequest = g.getResourceValue(res.Requests.CPU, cpuRequest)
			memoryRequest = g.getResourceValue(res.Requests.Memory, memoryRequest)
		}
		if res.Limits != nil {
			cpuLimit = g.getResourceValue(res.Limits.CPU, cpuLimit)
			memoryLimit = g.getResourceValue(res.Limits.Memory, memoryLimit)
		}
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: g.parseResourceQuantity(memoryRequest, constants.DefaultDBMemoryRequest),
			corev1.ResourceCPU:    g.parseResourceQuantity(cpuRequest, constants.DefaultDBCPURequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: g.parseResourceQuantity(memoryLimit, constants.DefaultDBMemoryLimit),
			corev1.ResourceCPU:    g.parseResourceQuantity(cpuLimit, constants.DefaultDBCPULimit),
		},
	}
}

// getKruizeResources returns Kruize application resource requirements with defaults
// For minikube/kind, defaults are disabled unless explicitly specified in the CR
func (g *KruizeResourceGenerator) getKruizeResources() corev1.ResourceRequirements {
	// For minikube/kind, only apply resources if explicitly specified in the CR
	if (g.ClusterType == constants.ClusterTypeMinikube || g.ClusterType == constants.ClusterTypeKind) {
		if g.KruizeSpec == nil || g.KruizeSpec.Kruize == nil || g.KruizeSpec.Kruize.Resources == nil {
			// Return empty resource requirements for minikube/kind when not specified
			return corev1.ResourceRequirements{}
		}
		// For minikube/kind with explicit resources, use only what's specified (no defaults)
		res := g.KruizeSpec.Kruize.Resources
		requirements := corev1.ResourceRequirements{}
		
		if res.Requests != nil {
			if res.Requests.CPU != "" || res.Requests.Memory != "" {
				requirements.Requests = corev1.ResourceList{}
				if res.Requests.CPU != "" {
					requirements.Requests[corev1.ResourceCPU] = resource.MustParse(res.Requests.CPU)
				}
				if res.Requests.Memory != "" {
					requirements.Requests[corev1.ResourceMemory] = resource.MustParse(res.Requests.Memory)
				}
			}
		}
		if res.Limits != nil {
			if res.Limits.CPU != "" || res.Limits.Memory != "" {
				requirements.Limits = corev1.ResourceList{}
				if res.Limits.CPU != "" {
					requirements.Limits[corev1.ResourceCPU] = resource.MustParse(res.Limits.CPU)
				}
				if res.Limits.Memory != "" {
					requirements.Limits[corev1.ResourceMemory] = resource.MustParse(res.Limits.Memory)
				}
			}
		}
		return requirements
	}

	// For OpenShift, use defaults
	cpuRequest := constants.DefaultKruizeCPURequest
	cpuLimit := constants.DefaultKruizeCPULimit
	memoryRequest := constants.DefaultKruizeMemoryRequest
	memoryLimit := constants.DefaultKruizeMemoryLimit

	if g.KruizeSpec != nil && g.KruizeSpec.Kruize != nil && g.KruizeSpec.Kruize.Resources != nil {
		res := g.KruizeSpec.Kruize.Resources
		if res.Requests != nil {
			cpuRequest = g.getResourceValue(res.Requests.CPU, cpuRequest)
			memoryRequest = g.getResourceValue(res.Requests.Memory, memoryRequest)
		}
		if res.Limits != nil {
			cpuLimit = g.getResourceValue(res.Limits.CPU, cpuLimit)
			memoryLimit = g.getResourceValue(res.Limits.Memory, memoryLimit)
		}
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: g.parseResourceQuantity(memoryRequest, constants.DefaultKruizeMemoryRequest),
			corev1.ResourceCPU:    g.parseResourceQuantity(cpuRequest, constants.DefaultKruizeCPURequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: g.parseResourceQuantity(memoryLimit, constants.DefaultKruizeMemoryLimit),
			corev1.ResourceCPU:    g.parseResourceQuantity(cpuLimit, constants.DefaultKruizeCPULimit),
		},
	}
}

// getDBVolumeMounts returns volume mounts for kruize-db with defaults
func (g *KruizeResourceGenerator) getDBVolumeMounts() []corev1.VolumeMount {
	defaultVolumeMounts := []corev1.VolumeMount{
		{Name: "kruize-db-storage", MountPath: "/var/lib/pgsql/data"},
	}

	if g.KruizeSpec != nil && g.KruizeSpec.KruizeDB != nil && len(g.KruizeSpec.KruizeDB.VolumeMounts) > 0 {
		volumeMounts := []corev1.VolumeMount{}
		for _, vm := range g.KruizeSpec.KruizeDB.VolumeMounts {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      vm.Name,
				MountPath: vm.MountPath,
			})
		}
		return volumeMounts
	}

	return defaultVolumeMounts
}

// getDBVolumes returns volumes for kruize-db pod with defaults
func (g *KruizeResourceGenerator) getDBVolumes() []corev1.Volume {
	pvcName := g.getPVCName("kruize-db-pv-claim")

	defaultVolumes := []corev1.Volume{
		{
			Name: "kruize-db-storage",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
	}

	if g.KruizeSpec != nil && g.KruizeSpec.KruizeDB != nil && len(g.KruizeSpec.KruizeDB.Volumes) > 0 {
		volumes := []corev1.Volume{}
		for _, v := range g.KruizeSpec.KruizeDB.Volumes {
			volume := corev1.Volume{
				Name: v.Name,
			}
			if v.PersistentVolumeClaim != nil {
				volume.VolumeSource = corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: v.PersistentVolumeClaim.ClaimName,
					},
				}
			}
			volumes = append(volumes, volume)
		}
		return volumes
	}

	return defaultVolumes
}

// getDBVolumeMountsKubernetes returns volume mounts for kruize-db with Kubernetes defaults
func (g *KruizeResourceGenerator) getDBVolumeMountsKubernetes() []corev1.VolumeMount {
	defaultVolumeMounts := []corev1.VolumeMount{
		{Name: "kruize-db-storage", MountPath: "/var/lib/postgresql/data"},
	}

	if g.KruizeSpec != nil && g.KruizeSpec.KruizeDB != nil && len(g.KruizeSpec.KruizeDB.VolumeMounts) > 0 {
		volumeMounts := []corev1.VolumeMount{}
		for _, vm := range g.KruizeSpec.KruizeDB.VolumeMounts {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      vm.Name,
				MountPath: vm.MountPath,
			})
		}
		return volumeMounts
	}

	return defaultVolumeMounts
}

// getDBVolumesKubernetes returns volumes for kruize-db pod with Kubernetes defaults
func (g *KruizeResourceGenerator) getDBVolumesKubernetes() []corev1.Volume {
	pvcName := g.getPVCName("kruize-db-pvc")

	defaultVolumes := []corev1.Volume{
		{
			Name: "kruize-db-storage",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
	}

	if g.KruizeSpec != nil && g.KruizeSpec.KruizeDB != nil && len(g.KruizeSpec.KruizeDB.Volumes) > 0 {
		volumes := []corev1.Volume{}
		for _, v := range g.KruizeSpec.KruizeDB.Volumes {
			volume := corev1.Volume{
				Name: v.Name,
			}
			if v.PersistentVolumeClaim != nil {
				volume.VolumeSource = corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: v.PersistentVolumeClaim.ClaimName,
					},
				}
			}
			volumes = append(volumes, volume)
		}
		return volumes
	}

	return defaultVolumes
}

// ensurePVAndPVCStorageConsistency validates and logs if PVC storage size is greater than PV storage size.
func ensurePVAndPVCStorageConsistency(pvStorageSize, pvcStorageSize string, ctx context.Context) (string, string, error) {
	logger := log.FromContext(ctx)
	
	pvQty, err := resource.ParseQuantity(pvStorageSize)
	if err != nil {
		return pvStorageSize, pvcStorageSize, fmt.Errorf("failed to parse PV storage size %q: %w", pvStorageSize, err)
	}

	pvcQty, err := resource.ParseQuantity(pvcStorageSize)
	if err != nil {
		return pvStorageSize, pvcStorageSize, fmt.Errorf("failed to parse PVC storage size %q: %w", pvcStorageSize, err)
	}

	// If PVC > PV, surface the potential bind-time failure.
	if pvcQty.Cmp(pvQty) == 1 {
		logger.Info("PVC storage request is greater than PV storage; PVC may fail to bind",
			"pvcStorageSize", pvcStorageSize,
			"pvStorageSize", pvStorageSize)
	}

	return pvStorageSize, pvcStorageSize, nil
}

// getPVConfigWithDefaults is a shared helper that returns PV configuration with injected defaults
func (g *KruizeResourceGenerator) getPVConfigWithDefaults(
	defaultPVStorageSize, defaultStorageClassName, defaultHostPath string,
	defaultAccessModes []corev1.PersistentVolumeAccessMode,
) (pvStorageSize, pvcStorageSize, storageClassName, hostPath string, accessModes []corev1.PersistentVolumeAccessMode) {
	pvStorageSize = defaultPVStorageSize
	pvcStorageSize = defaultPVStorageSize
	storageClassName = defaultStorageClassName
	hostPath = defaultHostPath
	accessModes = defaultAccessModes

	if g.KruizeSpec != nil {
		// Get PV configuration
		if g.KruizeSpec.PersistentVolume != nil {
			pv := g.KruizeSpec.PersistentVolume
			if pv.Capacity != nil && pv.Capacity.Storage != "" {
				pvStorageSize = pv.Capacity.Storage
			}
			storageClassName = g.getResourceValue(pv.StorageClassName, storageClassName)
			if pv.HostPath != nil && pv.HostPath.Path != "" {
				hostPath = pv.HostPath.Path
			}
			if len(pv.AccessModes) > 0 {
				accessModes = []corev1.PersistentVolumeAccessMode{}
				for _, mode := range pv.AccessModes {
					accessModes = append(accessModes, corev1.PersistentVolumeAccessMode(mode))
				}
			}
		}

		// Get PVC configuration
		if g.KruizeSpec.PersistentVolumeClaim != nil {
			pvc := g.KruizeSpec.PersistentVolumeClaim
			if pvc.Resources != nil && pvc.Resources.Requests != nil && pvc.Resources.Requests.Storage != "" {
				pvcStorageSize = pvc.Resources.Requests.Storage
			} else {
				pvcStorageSize = pvStorageSize
			}
		} else {
			pvcStorageSize = pvStorageSize
		}
	}

	// Validate PV/PVC storage consistency and log warnings if misconfigured
	pvStorageSize, pvcStorageSize, err := ensurePVAndPVCStorageConsistency(pvStorageSize, pvcStorageSize, g.Ctx)
	if err != nil {
		logger := log.FromContext(g.Ctx)
		logger.Info("Failed to validate PV/PVC storage consistency; continuing with provided values",
			"error", err.Error(),
			"pvStorageSize", pvStorageSize,
			"pvcStorageSize", pvcStorageSize)
	}

	return pvStorageSize, pvcStorageSize, storageClassName, hostPath, accessModes
}

// getPVName returns the PV name from spec or default
func (g *KruizeResourceGenerator) getPVName(defaultName string) string {
	if g.KruizeSpec != nil && g.KruizeSpec.PersistentVolume != nil && g.KruizeSpec.PersistentVolume.Name != "" {
		return g.KruizeSpec.PersistentVolume.Name
	}
	return defaultName
}

// getPVCName returns the PVC name from spec or default
func (g *KruizeResourceGenerator) getPVCName(defaultName string) string {
	if g.KruizeSpec != nil && g.KruizeSpec.PersistentVolumeClaim != nil && g.KruizeSpec.PersistentVolumeClaim.Name != "" {
		return g.KruizeSpec.PersistentVolumeClaim.Name
	}
	return defaultName
}

// getPVLabels returns the PV labels from spec or default
func (g *KruizeResourceGenerator) getPVLabels(defaultLabels map[string]string) map[string]string {
	if g.KruizeSpec != nil && g.KruizeSpec.PersistentVolume != nil && len(g.KruizeSpec.PersistentVolume.Labels) > 0 {
		return g.KruizeSpec.PersistentVolume.Labels
	}
	return defaultLabels
}

// getPVCLabels returns the PVC labels from spec or default
func (g *KruizeResourceGenerator) getPVCLabels(defaultLabels map[string]string) map[string]string {
	if g.KruizeSpec != nil && g.KruizeSpec.PersistentVolumeClaim != nil && len(g.KruizeSpec.PersistentVolumeClaim.Labels) > 0 {
		return g.KruizeSpec.PersistentVolumeClaim.Labels
	}
	return defaultLabels
}

// getPVConfig returns PV configuration with defaults for OpenShift
func (g *KruizeResourceGenerator) getPVConfig() (pvStorageSize, pvcStorageSize, storageClassName, hostPath string, accessModes []corev1.PersistentVolumeAccessMode) {
	return g.getPVConfigWithDefaults(
		constants.DefaultOpenShiftPVStorageSize,
		constants.DefaultOpenShiftStorageClassName,
		constants.DefaultOpenShiftHostPath,
		[]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
	)
}

// getPVConfigKubernetes returns PV configuration with defaults for Kubernetes/Kind/Minikube
func (g *KruizeResourceGenerator) getPVConfigKubernetes() (pvStorageSize, pvcStorageSize, storageClassName, hostPath string, accessModes []corev1.PersistentVolumeAccessMode) {
	return g.getPVConfigWithDefaults(
		constants.DefaultKubernetesPVStorageSize,
		constants.DefaultKubernetesStorageClassName,
		constants.DefaultKubernetesHostPath,
		[]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
	)
}

// ClusterScopedResources generates all cluster-scoped resources for Kruize.
// These resources DO NOT get an owner reference.
func (g *KruizeResourceGenerator) ClusterScopedResources() []client.Object {
	return []client.Object{
		g.recommendationUpdaterClusterRole(),
		g.recommendationUpdaterClusterRoleBinding(),
		g.monitoringViewClusterRoleBinding(),
		g.kruizeEditKOClusterRole(),
		g.instaslicesAccessClusterRole(),
		g.instaslicesAccessClusterRoleBinding(),
		g.kruizeEditKOClusterRoleBinding(),
		g.AutotuneClusterRoleBinding(),
		g.ManualStorageClass(),
		g.kruizeDBPersistentVolume(),
	}
}

// NamespacedResources generates all OpenShift namespaced resources for Kruize.
// These resources will get an owner reference set to the Kruize CR.
func (g *KruizeResourceGenerator) NamespacedResources() []client.Object {
	objects := []client.Object{
		g.kruizeDBPersistentVolumeClaim(),
		g.kruizeDBDeployment(),
		g.kruizeDBService(),
		g.kruizeDeployment(),
		g.kruizeService(),
		g.kruizeOptimizerDeployment(),
		g.kruizeOptimizerService(),
		g.createPartitionCronJob(),
		g.kruizeServiceMonitor(),
		g.nginxConfigMap(),
		g.kruizeUINginxService(),
		g.kruizeUINginxDeployment(),
		g.deletePartitionCronJob(),
	}

	objects = append(objects, g.Routes()...)
	return objects
}

func (g *KruizeResourceGenerator) Routes() []client.Object {
	routes := []*routev1.Route{
		g.generateRoute("kruize", "kruize", "kruize-port"),
		g.generateRoute("kruize-ui-nginx-service", "kruize-ui-nginx-service", "http"),
	}

	objects := make([]client.Object, len(routes))
	for i, route := range routes {
		objects[i] = route
	}
	return objects
}

// generateRoute is a private helper to avoid duplicating Route creation logic.
func (g *KruizeResourceGenerator) generateRoute(name, serviceName, targetPort string) *routev1.Route {
	weight := int32(100)
	return &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "route.openshift.io/v1",
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.Namespace,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   serviceName,
				Weight: &weight,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString(targetPort),
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}
}

// kruizeServiceAccount generates the ServiceAccount for Kruize.
func (g *KruizeResourceGenerator) KruizeServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-sa",
			Namespace: g.Namespace,
		},
	}
}

// recommendationUpdaterClusterRole generates the ClusterRole for Kruize recommendation updater.
func (g *KruizeResourceGenerator) recommendationUpdaterClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kruize-recommendation-updater",
		},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list", "watch", "create"}},
			{APIGroups: []string{""}, Resources: []string{"nodes", "namespaces", "services", "endpoints"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"apps"}, Resources: []string{"deployments", "replicasets", "statefulsets", "daemonsets"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"extensions", "networking.k8s.io"}, Resources: []string{"ingresses"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"metrics.k8s.io"}, Resources: []string{"pods", "nodes"}, Verbs: []string{"get", "list"}},
			{APIGroups: []string{"monitoring.coreos.com"}, Resources: []string{"prometheuses", "alertmanagers", "servicemonitors"}, Verbs: []string{"get", "list", "watch"}, ResourceNames: []string{"*"}},
			{APIGroups: []string{"monitoring.coreos.com"}, Resources: []string{"prometheuses/api"}, Verbs: []string{"get", "create", "update"}},
			{APIGroups: []string{"apiextensions.k8s.io"}, Resources: []string{"customresourcedefinitions"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"autoscaling.k8s.io"}, Resources: []string{"verticalpodautoscalers", "verticalpodautoscalers/status", "verticalpodautoscalercheckpoints"}, Verbs: []string{"get", "list", "watch", "create", "update", "patch"}},
			{APIGroups: []string{"rbac.authorization.k8s.io"}, Resources: []string{"clusterrolebindings"}, Verbs: []string{"get", "list", "watch", "create"}},
			{NonResourceURLs: []string{"/metrics", "/api/v1/label/*", "/api/v1/query*", "/api/v1/series*", "/api/v1/targets*"}, Verbs: []string{"get"}},
		},
	}
}

// recommendationUpdaterClusterRoleBinding generates the ClusterRoleBinding for the recommendation updater.
func (g *KruizeResourceGenerator) recommendationUpdaterClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kruize-recommendation-updater-crb",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "kruize-sa", Namespace: g.Namespace},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "kruize-recommendation-updater",
		},
	}
}

// monitoringViewClusterRoleBinding generates the ClusterRoleBinding for cluster monitoring view.
func (g *KruizeResourceGenerator) monitoringViewClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kruize-monitoring-view",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "kruize-sa", Namespace: g.Namespace},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-monitoring-view",
		},
	}
}

// AutotuneClusterRoleBinding generates the autotune-scc-crb ClusterRoleBinding
// This binds the kruize-sa ServiceAccount to the system:openshift:scc:anyuid ClusterRole
func (g *KruizeResourceGenerator) AutotuneClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "autotune-scc-crb",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:openshift:scc:anyuid",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "kruize-sa",
				Namespace: g.Namespace,
			},
		},
	}
}

// ManualStorageClass generates the manual StorageClass
// This StorageClass uses no provisioner and retains volumes
func (g *KruizeResourceGenerator) ManualStorageClass() *storagev1.StorageClass {
	reclaimPolicy := corev1.PersistentVolumeReclaimRetain
	volumeBindingMode := storagev1.VolumeBindingImmediate

	return &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.k8s.io/v1",
			Kind:       "StorageClass",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "manual",
		},
		Provisioner:       "kubernetes.io/no-provisioner",
		ReclaimPolicy:     &reclaimPolicy,
		VolumeBindingMode: &volumeBindingMode,
	}
}

// kruizeDBPersistentVolume generates the PersistentVolume for the Kruize database.
// Note: PersistentVolumes are cluster-scoped resources.
func (g *KruizeResourceGenerator) kruizeDBPersistentVolume() *corev1.PersistentVolume {
	pvStorageSize, _, storageClassName, hostPath, accessModes := g.getPVConfig()
	pvName := g.getPVName("kruize-db-pv-volume")
	labels := g.getPVLabels(map[string]string{
		"type": "local",
		"app":  "kruize-db",
	})

	return &corev1.PersistentVolume{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolume",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   pvName,
			Labels: labels,
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: storageClassName,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: g.parseResourceQuantity(pvStorageSize, constants.DefaultOpenShiftPVStorageSize),
			},
			AccessModes: accessModes,
			// The HostPath must be nested inside the PersistentVolumeSource struct.
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: hostPath,
				},
			},
		},
	}
}

// kruizeDBPersistentVolumeClaim generates the PersistentVolumeClaim for the Kruize database.
func (g *KruizeResourceGenerator) kruizeDBPersistentVolumeClaim() *corev1.PersistentVolumeClaim {
	_, pvcStorageSize, storageClassName, _, accessModes := g.getPVConfig()
	pvcName := g.getPVCName("kruize-db-pv-claim")
	labels := g.getPVCLabels(map[string]string{"app": "kruize-db"})

	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: g.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
			AccessModes:      accessModes,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: g.parseResourceQuantity(pvcStorageSize, constants.DefaultOpenShiftPVStorageSize),
				},
			},
		},
	}
}

// kruizeConfigMap generates the main ConfigMap for Kruize.
func (g *KruizeResourceGenerator) KruizeConfigMap() *corev1.ConfigMap {
	dbConfigJSON := `{
      "database": {
        "adminPassword": "admin",
        "adminUsername": "admin",
        "hostname": "kruize-db-service",
        "name": "kruizeDB",
        "password": "admin",
        "port": 5432,
        "sslMode": "disable",
        "username": "admin"
      }
    }`

	kruizeConfigJSON := fmt.Sprintf(`{
      "clustertype": "kubernetes",
      "k8stype": "openshift",
      "authtype": "",
      "monitoringagent": "prometheus",
      "monitoringservice": "prometheus-k8s",
      "monitoringendpoint": "prometheus-k8s",
      "savetodb": "true",
      "dbdriver": "jdbc:postgresql://",
      "plots": "true",
      "isROSEnabled": "false",
      "local": "true",
      "logAllHttpReqAndResp": "true",
      "recommendationsURL" : "http://kruize.%s.svc.cluster.local:8080/generateRecommendations?experiment_name=%%s",
      "experimentsURL" : "http://kruize.%s.svc.cluster.local:8080/createExperiment",
      "experimentNameFormat" : "%%datasource%%|%%clustername%%|%%namespace%%|%%workloadname%%(%%workloadtype%%)|%%containername%%",
      "bulkapilimit" : 1000,
      "isKafkaEnabled" : "false",
      "hibernate": {
        "dialect": "org.hibernate.dialect.PostgreSQLDialect",
        "driver": "org.postgresql.Driver",
        "c3p0minsize": 5,
        "c3p0maxsize": 10,
        "c3p0timeout": 300,
        "c3p0maxstatements": 100,
        "hbm2ddlauto": "none",
        "showsql": "false",
        "timezone": "UTC"
      },
      "logging" : {
        "cloudwatch": {
          "accessKeyId": "",
          "logGroup": "kruize-logs",
          "logStream": "kruize-stream",
          "region": "",
          "secretAccessKey": "",
          "logLevel": "INFO"
        }
      },
      "datasource": [
        {
          "name": "prometheus-1",
          "provider": "prometheus",
          "serviceName": "prometheus-k8s",
          "namespace": "openshift-monitoring",
          "url": "",
          "authentication": {
              "type": "bearer",
              "credentials": {
                "tokenFilePath": "/var/run/secrets/kubernetes.io/serviceaccount/token"
              }
          }
        },
        {
          "name": "thanos-1",
          "provider": "prometheus",
          "serviceName": "thanos-querier",
          "namespace": "openshift-monitoring",
          "url": "",
          "authentication": {
              "type": "bearer",
              "credentials": {
                "tokenFilePath": "/var/run/secrets/kubernetes.io/serviceaccount/token"
              }
          }
        }
      ]
    }`, g.Namespace, g.Namespace)

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruizeconfig",
			Namespace: g.Namespace,
		},
		Data: map[string]string{
			"dbconfigjson":     dbConfigJSON,
			"kruizeconfigjson": kruizeConfigJSON,
		},
	}
}

// kruizeDBDeployment is a private helper that generates the Deployment for the database.
func (g *KruizeResourceGenerator) kruizeDBDeployment() *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-db-deployment",
			Namespace: g.Namespace,
			Labels: map[string]string{
				"app": "kruize-db",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "kruize-db",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "kruize-db",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "kruize-sa",
					Containers: []corev1.Container{
						{
							Name:            "kruize-db",
							Image:           "quay.io/kruizehub/postgres:15.2",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{Name: "POSTGRES_PASSWORD", Value: "admin"},
								{Name: "POSTGRES_USER", Value: "admin"},
								{Name: "POSTGRES_DB", Value: "kruizeDB"},
								{Name: "PGDATA", Value: "/var/lib/pg_data"},
							},
							Resources: g.getDBResources(),
							Ports: []corev1.ContainerPort{
								{ContainerPort: 5432},
							},
							VolumeMounts: g.getDBVolumeMounts(),
						},
					},
					Volumes: g.getDBVolumes(),
				},
			},
		},
	}
}

// kruizeDBService is a private helper that generates the Service for the database.
func (g *KruizeResourceGenerator) kruizeDBService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-db-service",
			Namespace: g.Namespace,
			Labels: map[string]string{
				"app": "kruize-db",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "kruize-db-port",
					Port:       5432,
					TargetPort: intstr.FromInt(5432),
				},
			},
			Selector: map[string]string{
				"app": "kruize-db",
			},
		},
	}
}

// kruizeDeployment is a private helper that generates the deployment for the Kruize backend.
func (g *KruizeResourceGenerator) kruizeDeployment() *appsv1.Deployment {
	replicas := int32(1)

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize",
			Namespace: g.Namespace,
			Labels: map[string]string{
				"app": "kruize",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "kruize",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":  "kruize",
						"name": "kruize",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "kruize-sa",
					InitContainers: []corev1.Container{
						{
							Name:            "wait-for-kruize-db",
							Image:           "quay.io/kruizehub/postgres:15.2",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"sh",
								"-c",
								"until pg_isready -h kruize-db-service -p 5432 -U admin; do\n  echo \"Waiting for kruize-db-service to be ready...\"\n  sleep 2\ndone\n",
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "kruize",
							Image:           g.Autotune_image,
							ImagePullPolicy: corev1.PullAlways,
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config-volume", MountPath: "/etc/config"},
							},
							Env: []corev1.EnvVar{
								{Name: "LOGGING_LEVEL", Value: "info"},
								{Name: "ROOT_LOGGING_LEVEL", Value: "error"},
								{Name: "DB_CONFIG_FILE", Value: "/etc/config/dbconfigjson"},
								{Name: "KRUIZE_CONFIG_FILE", Value: "/etc/config/kruizeconfigjson"},
								{Name: "JAVA_TOOL_OPTIONS", Value: "-XX:MaxRAMPercentage=80"},
								{Name: "KAFKA_BOOTSTRAP_SERVERS", Value: "kruize-kafka-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092"},
								{Name: "KAFKA_TOPICS", Value: "recommendations-topic,error-topic,summary-topic"},
								{Name: "KAFKA_RESPONSE_FILTER_INCLUDE", Value: "experiments|status|apis|recommendations|response|status_history"},
								{Name: "KAFKA_RESPONSE_FILTER_EXCLUDE", Value: ""},
							},
							Resources: g.getKruizeResources(),
							Ports: []corev1.ContainerPort{
								{Name: "kruize-port", ContainerPort: 8080},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kruizeconfig",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// kruizeService is a private helper that generates the Service for the Kruize backend.
func (g *KruizeResourceGenerator) kruizeService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize",
			Namespace: g.Namespace,
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/path":   "/metrics",
			},
			Labels: map[string]string{
				"app": "kruize",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "kruize",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "kruize-port",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
}

// kruizeOptimizerDeployment generates the Deployment for the Kruize Optimizer.
func (g *KruizeResourceGenerator) kruizeOptimizerDeployment() *appsv1.Deployment {
	replicas := int32(1)

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-optimizer",
			Namespace: g.Namespace,
			Labels: map[string]string{
				"app": "kruize-optimizer",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "kruize-optimizer",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "kruize-optimizer",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "kruize-optimizer",
							Image:           g.Optimizer_image,
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080},
							},
							Env: []corev1.EnvVar{
								{Name: "ENABLE_SWAGGER", Value: "true"},
								{Name: "KRUIZE_URL", Value: "http://kruize:8080"},
								{Name: "KRUIZE_STATE_REFRESH_INTERVAL", Value: "60m"},
								{Name: "KRUIZE_BULK_SCHEDULER_INTERVAL", Value: "15m"},
								{Name: "KRUIZE_BULK_SCHEDULER_STARTUP_DELAY", Value: "1m"},
								{Name: "KRUIZE_BULK_MEASUREMENT_DURATION", Value: "15min"},
								{Name: "KRUIZE_WEBHOOK_URL", Value: "http://kruize-optimizer:8080/webhook"},
								{Name: "KRUIZE_TARGET_LABEL_LIMIT", Value: "1"},
								{Name: "KRUIZE_TARGET_LABELS", Value: `{"kruize/autotune": "enabled"}`},
								{Name: "KRUIZE_DEFAULT_DATASOURCE", Value: "prometheus-1"},
								{Name: "KRUIZE_DEFAULT_METADATA_PROFILE", Value: "cluster-metadata-local-monitoring"},
								{Name: "KRUIZE_DEFAULT_METRIC_PROFILE", Value: "resource-optimization-local-monitoring"},
							},
						},
					},
				},
			},
		},
	}
}

// kruizeOptimizerService generates the Service for the Kruize Optimizer.
func (g *KruizeResourceGenerator) kruizeOptimizerService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-optimizer",
			Namespace: g.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": "kruize-optimizer",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
}

func (g *KruizeResourceGenerator) kruizeUINginxDeployment() *appsv1.Deployment {
	replicas := int32(1)
	
	// Build pod security context based on cluster type
	podSecurityContext := &corev1.PodSecurityContext{
		RunAsNonRoot: boolPtr(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
	
	// Only set RunAsUser for non-OpenShift clusters
	// OpenShift SCC will reject hardcoded UIDs and assign its own
	if g.ClusterType != constants.ClusterTypeOpenShift {
		podSecurityContext.RunAsUser = int64Ptr(101)
	}
	
	return &appsv1.Deployment{
		// The TypeMeta tells the client which kind of object this is.
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		// The ObjectMeta contains the name, namespace, and labels.
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-ui-nginx",
			Namespace: g.Namespace,
			Labels: map[string]string{
				"app": "kruize-ui-nginx",
			},
		},
		// The Spec defines the desired state of the Deployment.
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "kruize-ui-nginx",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "kruize-ui-nginx",
					},
				},
				Spec: corev1.PodSpec{
					SecurityContext: podSecurityContext,
					Containers: []corev1.Container{
						{
							Name:            "kruize-ui-nginx-container",
							Image:           g.Autotune_ui_image,
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								{Name: "KRUIZE_UI_ENV", Value: "production"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "nginx-config-volume",
									MountPath: "/etc/nginx/nginx.conf",
									SubPath:   "nginx.conf",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "nginx-config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "nginx-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// nginxConfigMap is an internal helper that generates the ConfigMap for the Nginx configuration.
func (g *KruizeResourceGenerator) nginxConfigMap() *corev1.ConfigMap {
	nginxConf := `
events {}
pid /tmp/nginx.pid;
http {
	 client_body_temp_path /tmp/client_temp;
	 proxy_temp_path /tmp/proxy_temp;
	 fastcgi_temp_path /tmp/fastcgi_temp;
	 uwsgi_temp_path /tmp/uwsgi_temp;
	 scgi_temp_path /tmp/scgi_temp;

	 upstream kruize-api {
	   server kruize:8080;
	 }

	 server {
	   listen 8080;
	   server_name localhost;

	   root   /usr/share/nginx/html;

	   location ^~ /api/ {
	     rewrite ^/api(.*)$ $1 break;
	     proxy_pass http://kruize-api;
	   }

	   location / {
	     index index.html;
	     error_page 404 =200 /index.html;
	   }
	 }
}
`
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-config",
			Namespace: g.Namespace,
		},
		Data: map[string]string{
			"nginx.conf": nginxConf,
		},
	}
}

// kruizeUINginxService is an internal helper that generates the Service for the Kruize UI Nginx pod.
func (g *KruizeResourceGenerator) kruizeUINginxService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-ui-nginx-service",
			Namespace: g.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Selector: map[string]string{
				"app": "kruize-ui-nginx",
			},
		},
	}
}

// ============================================================================
// Kind/Minikube Resources (from minikube/kind YAML)
// ============================================================================

// kruizeEditKOClusterRole generates the ClusterRole for editing Kruize resources
func (g *KruizeResourceGenerator) kruizeEditKOClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kruize-edit-ko",
		},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{"apps"}, Resources: []string{"deployments", "statefulsets", "daemonsets"}, Verbs: []string{"get", "list"}},
			{APIGroups: []string{"batch"}, Resources: []string{"jobs"}, Verbs: []string{"get", "list"}},
			{APIGroups: []string{""}, Resources: []string{"namespaces"}, Verbs: []string{"get", "list"}},
		},
	}
}

// kruizeEditKOClusterRoleBinding generates the ClusterRoleBinding for kruize-edit-ko
func (g *KruizeResourceGenerator) kruizeEditKOClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kruize-edit-ko-binding",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "kruize-sa", Namespace: g.Namespace},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "kruize-edit-ko",
		},
	}
}

// kruizeEditKOClusterRoleBindingKubernetes generates the ClusterRoleBinding for kruize-edit-ko
func (g *KruizeResourceGenerator) kruizeEditKOClusterRoleBindingKubernetes() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kruize-edit-ko-binding",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "default", Namespace: g.Namespace},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "kruize-edit-ko",
		},
	}
}

// instaslicesAccessClusterRole generates the ClusterRole for instaslices access
func (g *KruizeResourceGenerator) instaslicesAccessClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "instaslices-access",
		},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{"inference.redhat.com"}, Resources: []string{"instaslices"}, Verbs: []string{"get", "list", "watch"}},
		},
	}
}

// instaslicesAccessClusterRoleBinding generates the ClusterRoleBinding for instaslices access
func (g *KruizeResourceGenerator) instaslicesAccessClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "instaslices-access-binding",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "kruize-sa", Namespace: g.Namespace},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "instaslices-access",
		},
	}
}

// instaslicesAccessClusterRoleBindingKubernetes generates the ClusterRoleBinding for instaslices access
func (g *KruizeResourceGenerator) instaslicesAccessClusterRoleBindingKubernetes() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "instaslices-access-binding",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "default", Namespace: g.Namespace},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "instaslices-access",
		},
	}
}

// kruizeDBPersistentVolumeKubernetes generates PV for Kind/Minikube/Kubernetes (different from OpenShift)
func (g *KruizeResourceGenerator) kruizeDBPersistentVolumeKubernetes() *corev1.PersistentVolume {
	pvStorageSize, _, storageClassName, hostPath, accessModes := g.getPVConfigKubernetes()
	pvName := g.getPVName("kruize-db-pv")
	labels := g.getPVLabels(map[string]string{"app": "kruize-db"})

	return &corev1.PersistentVolume{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolume",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   pvName,
			Labels: labels,
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: storageClassName,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: g.parseResourceQuantity(pvStorageSize, constants.DefaultKubernetesPVStorageSize),
			},
			AccessModes:                   accessModes,
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: hostPath,
				},
			},
		},
	}
}

// kruizeDBPersistentVolumeClaimKubernetes generates PVC for Kind/Minikube/Kubernetes
func (g *KruizeResourceGenerator) kruizeDBPersistentVolumeClaimKubernetes() *corev1.PersistentVolumeClaim {
	_, pvcStorageSize, storageClassName, _, accessModes := g.getPVConfigKubernetes()
	pvcName := g.getPVCName("kruize-db-pvc")
	labels := g.getPVCLabels(map[string]string{"app": "kruize-db"})

	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: g.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
			AccessModes:      accessModes,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: g.parseResourceQuantity(pvcStorageSize, constants.DefaultKubernetesPVStorageSize),
				},
			},
		},
	}
}

// kruizeDBDeploymentKubernetes generates DB deployment for Kind/Minikube with init container
func (g *KruizeResourceGenerator) kruizeDBDeploymentKubernetes() *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-db-deployment",
			Namespace: g.Namespace,
			Labels: map[string]string{
				"app": "kruize-db",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "kruize-db",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "kruize-db",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "kruize-db",
							Image:           "quay.io/kruizehub/postgres:15.2",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{Name: "POSTGRES_PASSWORD", Value: "admin"},
								{Name: "POSTGRES_USER", Value: "admin"},
								{Name: "POSTGRES_DB", Value: "kruizeDB"},
								{Name: "PGDATA", Value: "/var/lib/postgresql/data/pgdata"},
							},
							Resources: g.getDBResources(),
							Ports: []corev1.ContainerPort{
								{ContainerPort: 5432},
							},
							VolumeMounts: g.getDBVolumeMountsKubernetes(),
						},
					},
					Volumes: g.getDBVolumesKubernetes(),
				},
			},
		},
	}
}

// kruizeConfigMapKubernetes generates ConfigMap for Kind/Minikube/Kubernetes
func (g *KruizeResourceGenerator) KruizeConfigMapKubernetes() *corev1.ConfigMap {
	dbConfigJSON := `{
	     "database": {
	       "adminPassword": "admin",
	       "adminUsername": "admin",
	       "hostname": "kruize-db-service",
	       "name": "kruizeDB",
	       "password": "admin",
	       "port": 5432,
	       "sslMode": "require",
	       "username": "admin"
	     }
	   }`

	kruizeConfigJSON := fmt.Sprintf(`{
	     "clustertype": "kubernetes",
	     "k8stype": "minikube",
	     "authtype": "",
	     "monitoringagent": "prometheus",
	     "monitoringservice": "prometheus-k8s",
	     "monitoringendpoint": "prometheus-k8s",
	     "savetodb": "true",
	     "dbdriver": "jdbc:postgresql://",
	     "plots": "true",
	     "logAllHttpReqAndResp": "true",
	     "recommendationsURL" : "http://kruize.%s.svc.cluster.local:8080/generateRecommendations?experiment_name=%%s",
	     "experimentsURL" : "http://kruize.%s.svc.cluster.local:8080/createExperiment",
	     "experimentNameFormat" : "%%datasource%%|%%clustername%%|%%namespace%%|%%workloadname%%(%%workloadtype%%)|%%containername%%",
	     "bulkapilimit" : 1000,
	     "isKafkaEnabled" : "false",
	     "isROSEnabled": "false",
	     "local": "true",
	     "hibernate": {
	       "dialect": "org.hibernate.dialect.PostgreSQLDialect",
	       "driver": "org.postgresql.Driver",
	       "c3p0minsize": 2,
	       "c3p0maxsize": 5,
	       "c3p0timeout": 300,
	       "c3p0maxstatements": 50,
	       "hbm2ddlauto": "none",
	       "showsql": "false",
	       "timezone": "UTC"
	     },
	     "logging" : {
	       "cloudwatch": {
	         "accessKeyId": "",
	         "logGroup": "kruize-logs",
	         "logStream": "kruize-stream",
	         "region": "",
	         "secretAccessKey": "",
	         "logLevel": "INFO"
	       }
	     },
	     "datasource": [
	       {
	         "name": "prometheus-1",
	         "provider": "prometheus",
	         "serviceName": "prometheus-k8s",
	         "namespace": "monitoring",
	         "url": ""
	       }
	     ]
	   }`, g.Namespace, g.Namespace)

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruizeconfig",
			Namespace: g.Namespace,
		},
		Data: map[string]string{
			"dbconfigjson":     dbConfigJSON,
			"kruizeconfigjson": kruizeConfigJSON,
		},
	}
}

// kruizeDeploymentKubernetes generates Kruize deployment for Kind/Minikube with init container
func (g *KruizeResourceGenerator) kruizeDeploymentKubernetes() *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize",
			Namespace: g.Namespace,
			Labels: map[string]string{
				"app": "kruize",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "kruize",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":  "kruize",
						"name": "kruize",
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:            "wait-for-kruize-db",
							Image:           "quay.io/kruizehub/postgres:15.2",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"sh",
								"-c",
								"until pg_isready -h kruize-db-service -p 5432 -U admin; do echo \"Waiting for kruize-db-service to be ready...\"; sleep 2; done",
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "kruize",
							Image:           g.Autotune_image,
							ImagePullPolicy: corev1.PullAlways,
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config-volume", MountPath: "/etc/config"},
							},
							Env: []corev1.EnvVar{
								{Name: "LOGGING_LEVEL", Value: "info"},
								{Name: "ROOT_LOGGING_LEVEL", Value: "error"},
								{Name: "DB_CONFIG_FILE", Value: "/etc/config/dbconfigjson"},
								{Name: "KRUIZE_CONFIG_FILE", Value: "/etc/config/kruizeconfigjson"},
								{Name: "JAVA_TOOL_OPTIONS", Value: "-XX:MaxRAMPercentage=80"},
								{Name: "KAFKA_BOOTSTRAP_SERVERS", Value: "kruize-kafka-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092"},
								{Name: "KAFKA_TOPICS", Value: "recommendations-topic,error-topic,summary-topic"},
								{Name: "KAFKA_RESPONSE_FILTER_INCLUDE", Value: "summary"},
								{Name: "KAFKA_RESPONSE_FILTER_EXCLUDE", Value: ""},
							},
							Resources: g.getKruizeResources(),
							Ports: []corev1.ContainerPort{
								{Name: "kruize-port", ContainerPort: 8080},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kruizeconfig",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// kruizeServiceKubernetes generates Service for Kind/Minikube (NodePort instead of ClusterIP)
func (g *KruizeResourceGenerator) kruizeServiceKubernetes() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize",
			Namespace: g.Namespace,
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/path":   "/metrics",
			},
			Labels: map[string]string{
				"app": "kruize",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "kruize",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "kruize-port",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
}

// createPartitionCronJob generates the CronJob for creating partitions
func (g *KruizeResourceGenerator) createPartitionCronJob() *batchv1.CronJob {
	return &batchv1.CronJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "CronJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "create-partition-cronjob",
			Namespace: g.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 0 25 * *", // Run on 25th of every month at midnight
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            "kruizecronjob",
									Image:           g.Autotune_image,
									ImagePullPolicy: corev1.PullAlways,
									VolumeMounts: []corev1.VolumeMount{
										{Name: "config-volume", MountPath: "/etc/config"},
									},
									Command: []string{"sh", "-c", "/home/autotune/app/target/bin/CreatePartition"},
									Args:    []string{""},
									Env: []corev1.EnvVar{
										{Name: "START_AUTOTUNE", Value: "false"},
										{Name: "LOGGING_LEVEL", Value: "info"},
										{Name: "ROOT_LOGGING_LEVEL", Value: "error"},
										{Name: "DB_CONFIG_FILE", Value: "/etc/config/dbconfigjson"},
										{Name: "KRUIZE_CONFIG_FILE", Value: "/etc/config/kruizeconfigjson"},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "config-volume",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "kruizeconfig",
											},
										},
									},
								},
							},
							RestartPolicy: corev1.RestartPolicyOnFailure,
						},
					},
				},
			},
		},
	}
}

// deletePartitionCronJob generates the CronJob for deleting old partitions
func (g *KruizeResourceGenerator) deletePartitionCronJob() *batchv1.CronJob {
	return &batchv1.CronJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "CronJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-delete-partition-cronjob",
			Namespace: g.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 0 25 * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            "kruizedeletejob",
									Image:           g.Autotune_image,
									ImagePullPolicy: corev1.PullAlways,
									VolumeMounts: []corev1.VolumeMount{
										{Name: "config-volume", MountPath: "/etc/config"},
									},
									Command: []string{"sh", "-c", "/home/autotune/app/target/bin/RetentionPartition"},
									Args:    []string{""},
									Env: []corev1.EnvVar{
										{Name: "START_AUTOTUNE", Value: "false"},
										{Name: "LOGGING_LEVEL", Value: "info"},
										{Name: "ROOT_LOGGING_LEVEL", Value: "error"},
										{Name: "DB_CONFIG_FILE", Value: "/etc/config/dbconfigjson"},
										{Name: "KRUIZE_CONFIG_FILE", Value: "/etc/config/kruizeconfigjson"},
										{Name: "deletepartitionsthreshold", Value: "15"},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "config-volume",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "kruizeconfig",
											},
										},
									},
								},
							},
							RestartPolicy: corev1.RestartPolicyOnFailure,
						},
					},
				},
			},
		},
	}
}

// kruizeServiceMonitor generates the ServiceMonitor for Prometheus monitoring
func (g *KruizeResourceGenerator) kruizeServiceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.coreos.com/v1",
			Kind:       "ServiceMonitor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-service-monitor",
			Namespace: g.Namespace,
			Labels: map[string]string{
				"app": "kruize",
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "kruize",
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port:     "kruize-port",
					Interval: "30s",
					Path:     "/metrics",
				},
			},
		},
	}
}

// kruizeToPrometheusNetworkPolicy generates a NetworkPolicy to allow Kruize pods to access Prometheus
func (g *KruizeResourceGenerator) kruizeToPrometheusNetworkPolicy() *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-to-prometheus",
			Namespace: g.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "prometheus",
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "kruize",
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: func() *corev1.Protocol { p := corev1.ProtocolTCP; return &p }(),
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 9090},
						},
					},
				},
			},
		},
	}
}

// recommendationUpdaterClusterRoleBindingKubernetes generates the ClusterRoleBinding for the recommendation updater.
func (g *KruizeResourceGenerator) recommendationUpdaterClusterRoleBindingKubernetes() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kruize-recommendation-updater-crb",
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "default", Namespace: g.Namespace},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "kruize-recommendation-updater",
		},
	}
}

// KubernetesClusterScopedResources returns cluster-scoped resources for Kind/Minikube/Kubernetes
func (g *KruizeResourceGenerator) KubernetesClusterScopedResources() []client.Object {
	return []client.Object{
		g.recommendationUpdaterClusterRole(),
		g.recommendationUpdaterClusterRoleBindingKubernetes(),
		g.kruizeEditKOClusterRole(),
		g.instaslicesAccessClusterRole(),
		g.instaslicesAccessClusterRoleBindingKubernetes(),
		g.kruizeEditKOClusterRoleBindingKubernetes(),
		g.kruizeDBPersistentVolumeKubernetes(),
	}
}

// KubernetesNamespacedResources returns namespaced resources for Kind/minikube/Kubernetes
func (g *KruizeResourceGenerator) KubernetesNamespacedResources() []client.Object {
	return []client.Object{
		g.kruizeDBPersistentVolumeClaimKubernetes(),
		g.kruizeToPrometheusNetworkPolicy(),
		g.kruizeDBDeploymentKubernetes(),
		g.kruizeDBService(),
		g.kruizeDeploymentKubernetes(),
		g.kruizeServiceKubernetes(),
		g.kruizeOptimizerDeployment(),
		g.kruizeOptimizerService(),
		g.createPartitionCronJob(),
		g.kruizeServiceMonitor(),
		g.nginxConfigMap(),
		g.kruizeUINginxService(),
		g.kruizeUINginxDeployment(),
		g.deletePartitionCronJob(),
	}
}
