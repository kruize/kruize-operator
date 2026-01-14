package utils

import (
	"fmt"

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
}

// NewKruizeResourceGenerator creates a new generator for Kruize resources.
func NewKruizeResourceGenerator(namespace string, autotuneImage string, autotuneUIImage string, optimizerImage string, clusterType string) *KruizeResourceGenerator {
	// If no image is provided from the CR, use a sensible default.
	if autotuneImage == "" {
		autotuneImage = "quay.io/kruize/autotune_operator:latest"
	}
	if autotuneUIImage == "" {
		autotuneUIImage = "quay.io/kruize/kruize-ui:0.0.9"
	}
	if optimizerImage == "" {
		optimizerImage = "quay.io/rh-ee-shesaxen/optimizer:pocv5"
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
	}
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
		g.OptimizerClusterRole(),
		g.OptimizerClusterRoleBinding(),
		g.ManualStorageClass(),
		g.kruizeDBPersistentVolume(),
		g.kruizeDBPersistentVolumeClaim(),
	}
}

// NamespacedResources generates all OpenShift namespaced resources for Kruize.
// These resources will get an owner reference set to the Kruize CR.
func (g *KruizeResourceGenerator) NamespacedResources() []client.Object {
	objects := []client.Object{
		g.kruizeDBDeployment(),
		g.kruizeDBService(),
		g.kruizeDeployment(),
		g.kruizeService(),
		g.OptimizerServiceAccount(),
		g.OptimizerDeployment(),
		g.OptimizerService(),
		g.createPartitionCronJob(),
		g.kruizeServiceMonitor(),
		g.nginxConfigMap(),
		g.kruizeUINginxService(),
		g.kruizeUINginxPod(),
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
func (g *KruizeResourceGenerator) KruizeNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: g.Namespace,
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
			{APIGroups: []string{""}, Resources: []string{"pods", "nodes", "namespaces", "services", "endpoints"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"apps"}, Resources: []string{"deployments", "replicasets", "statefulsets", "daemonsets"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"extensions", "networking.k8s.io"}, Resources: []string{"ingresses"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"autoscaling.k8s.io"}, Resources: []string{"verticalpodautoscalers"}, Verbs: []string{"get", "list", "watch", "create", "update", "patch"}},
			{APIGroups: []string{"metrics.k8s.io"}, Resources: []string{"pods", "nodes"}, Verbs: []string{"get", "list"}},
			{APIGroups: []string{"monitoring.coreos.com"}, Resources: []string{"prometheuses", "alertmanagers", "servicemonitors"}, Verbs: []string{"get", "list", "watch"}, ResourceNames: []string{"*"}},
			{APIGroups: []string{"monitoring.coreos.com"}, Resources: []string{"prometheuses/api"}, Verbs: []string{"get", "create", "update"}},
			{APIGroups: []string{"apiextensions.k8s.io"}, Resources: []string{"customresourcedefinitions"}, Verbs: []string{"get", "list", "watch", "create"}},
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
	return &corev1.PersistentVolume{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolume",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kruize-db-pv-volume",
			Labels: map[string]string{
				"type": "local",
				"app":  "kruize-db",
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: "manual",
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("500Mi"),
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			// The HostPath must be nested inside the PersistentVolumeSource struct.
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/mnt/data",
				},
			},
		},
	}
}

// kruizeDBPersistentVolumeClaim generates the PersistentVolumeClaim for the Kruize database.
func (g *KruizeResourceGenerator) kruizeDBPersistentVolumeClaim() *corev1.PersistentVolumeClaim {
	storageClassName := "manual"
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-db-pv-claim",
			Namespace: g.Namespace,
			Labels: map[string]string{
				"app": "kruize-db",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("500Mi"),
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
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("100Mi"),
									corev1.ResourceCPU:    resource.MustParse("0.5"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("100Mi"),
									corev1.ResourceCPU:    resource.MustParse("0.5"),
								},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 5432},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "kruize-db-storage", MountPath: "/var/lib/pgsql/data"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "kruize-db-storage",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "kruize-db-pv-claim",
								},
							},
						},
					},
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
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("768Mi"),
									corev1.ResourceCPU:    resource.MustParse("0.7"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("768Mi"),
									corev1.ResourceCPU:    resource.MustParse("0.7"),
								},
							},
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

func (g *KruizeResourceGenerator) kruizeUINginxPod() *corev1.Pod {
	return &corev1.Pod{
		// The TypeMeta tells the client which kind of object this is.
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		// The ObjectMeta contains the name, namespace, and labels.
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-ui-nginx-pod",
			Namespace: g.Namespace, // We use the namespace from our generator struct.
			Labels: map[string]string{
				"app": "kruize-ui-nginx",
			},
		},
		// The Spec defines the desired state of the Pod.
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:            "wait-for-kruize-optimizer",
					Image:           "curlimages/curl:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"sh",
						"-c",
						"until curl --head --fail --silent http://kruize-optimizer:8080; do echo 'Waiting for Kruize Optimizer...'; sleep 2; done",
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "kruize-ui-nginx-container",
					Image:           g.Autotune_ui_image,
					ImagePullPolicy: corev1.PullAlways,
					Env: []corev1.EnvVar{
						{Name: "KRUIZE_API_URL", Value: "http://kruize:8080"},
						{Name: "REACT_APP_KRUIZE_API_URL", Value: "http://kruize:8080"},
						{Name: "KRUIZE_UI_API_URL", Value: "http://kruize:8080"},
						{Name: "API_URL", Value: "http://kruize:8080"},
						{Name: "KRUIZE_UI_ENV", Value: "production"},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "nginx-config-volume",
							MountPath: "/etc/nginx/nginx.conf",
							SubPath:   "nginx.conf",
						},
						{
							Name:      "nginx-cache",
							MountPath: "/var/cache/nginx",
						},
						{Name: "nginx-pid", MountPath: "/var/run"},
						{Name: "nginx-tmp", MountPath: "/tmp"},
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: boolPtr(false),
						RunAsNonRoot:             boolPtr(true),
						RunAsUser:                int64Ptr(101),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
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
				// Define the emptyDir volume that will be used for caching.
				{
					Name: "nginx-cache",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{Name: "nginx-pid", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "nginx-tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
		},
	}
}

// nginxConfigMap is an internal helper that generates the ConfigMap for the Nginx configuration.
func (g *KruizeResourceGenerator) nginxConfigMap() *corev1.ConfigMap {
	nginxConf := fmt.Sprintf(`
events {}
http {
  include       /etc/nginx/mime.types;
  default_type  application/octet-stream;

  # 1. Define the Upstreams (Backends)
  upstream kruize-api {
	server kruize:8080;
  }

  # NEW: Define where the Optimizer lives
  upstream kruize-optimizer-api {
	server kruize-optimizer.%s.svc.cluster.local:8080;
  }

  server {
	listen 8080;
	server_name localhost;
	root   /usr/share/nginx/html;
	index index.html;

	# 2. Proxy /api to Kruize
	location ^~ /api/ {
	  rewrite ^/api(.*)$ $1 break;
	  proxy_pass http://kruize-api;
	}

	# 3. NEW: Proxy /optimizer-api to Optimizer
	location ^~ /optimizer-api/ {
	  rewrite ^/optimizer-api(.*)$ $1 break;
	  proxy_pass http://kruize-optimizer-api;
	}

	# 4. Handle Client-Side Routing (Fixes 404 on refresh)
	location / {
	  try_files $uri $uri/ /index.html;
	}
  }
}
`, g.Namespace)
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
	return &corev1.PersistentVolume{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolume",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-db-pv",
			Namespace: g.Namespace,
			Labels: map[string]string{
				"app": "kruize-db",
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/data/postgres",
				},
			},
		},
	}
}

// kruizeDBPersistentVolumeClaimKubernetes generates PVC for Kind/Minikube/Kubernetes
func (g *KruizeResourceGenerator) kruizeDBPersistentVolumeClaimKubernetes() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-db-pvc",
			Namespace: g.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
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
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 5432},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "kruize-db-storage", MountPath: "/var/lib/postgresql/data"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "kruize-db-storage",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "kruize-db-pvc",
								},
							},
						},
					},
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
		g.OptimizerClusterRole(),
		g.OptimizerClusterRoleBinding(),
		g.kruizeDBPersistentVolumeKubernetes(),
		g.kruizeDBPersistentVolumeClaimKubernetes(),
	}
}

// KubernetesNamespacedResources returns namespaced resources for Kind/minikube/Kubernetes
func (g *KruizeResourceGenerator) KubernetesNamespacedResources() []client.Object {
	return []client.Object{
		g.kruizeToPrometheusNetworkPolicy(),
		g.kruizeDBDeploymentKubernetes(),
		g.kruizeDBService(),
		g.kruizeDeploymentKubernetes(),
		g.kruizeServiceKubernetes(),
		g.OptimizerServiceAccount(),
		g.OptimizerDeployment(),
		g.OptimizerService(),
		g.createPartitionCronJob(),
		g.kruizeServiceMonitor(),
		g.nginxConfigMap(),
		g.kruizeUINginxService(),
		g.kruizeUINginxPod(),
		g.deletePartitionCronJob(),
	}
}

// ============================================================================
// Optimizer Resources
// ============================================================================

func (g *KruizeResourceGenerator) OptimizerServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kruize-optimizer-sa",
			Namespace: g.Namespace,
		},
	}
}

func (g *KruizeResourceGenerator) OptimizerClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kruize-optimizer-role",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets", "replicasets"},
				Verbs:     []string{"get", "list", "watch", "patch", "update"},
			},
		},
	}
}

func (g *KruizeResourceGenerator) OptimizerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kruize-optimizer-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "kruize-optimizer-sa",
				Namespace: g.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "kruize-optimizer-role",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

func (g *KruizeResourceGenerator) OptimizerDeployment() *appsv1.Deployment {
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
					ServiceAccountName: "kruize-optimizer-sa",
					InitContainers: []corev1.Container{
						{
							Name:            "wait-for-kruize",
							Image:           "curlimages/curl:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"sh",
								"-c",
								"until curl --head --fail --silent http://kruize:8080; do echo 'Waiting for Kruize...'; sleep 2; done",
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "kruize-optimizer",
							Image:           g.Optimizer_image,
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080},
							},
							Env: []corev1.EnvVar{
								{Name: "KRUIZE_SCAN_INTERVAL", Value: "m"},
								{Name: "KRUIZE_JOB_POLLING_INTERVAL", Value: "1m"},
								{Name: "KRUIZE_URL", Value: fmt.Sprintf("http://kruize.%s.svc.cluster.local:8080", g.Namespace)},
								{Name: "KRUIZE_PROFILE_GIT_URL", Value: "local"},
								{Name: "KRUIZE_PROFILE_GIT_BASE_URL", Value: "local"},
								{Name: "KRUIZE_TARGET_LABEL_LIMIT", Value: "5"},
								{Name: "KRUIZE_TARGET_LABELS", Value: "[\"kruize/autotune=enabled\"]"},
							},
						},
					},
				},
			},
		},
	}
}

func (g *KruizeResourceGenerator) OptimizerService() *corev1.Service {
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
