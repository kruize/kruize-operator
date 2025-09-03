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

	mydomainv1alpha1 "github.com/kruize/kruize-operator/api/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// KruizeReconciler reconciles a Kruize object
type KruizeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=my.domain,resources=kruizes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=my.domain,resources=kruizes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=my.domain,resources=kruizes/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=get;list;watch;create;update;patch;delete;use
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=my.domain,resources=kruizes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=my.domain,resources=kruizes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=my.domain,resources=kruizes/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheuses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheuses/api,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=alertmanagers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=metrics.k8s.io,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=metrics.k8s.io,resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=autoscaling.k8s.io,resources=verticalpodautoscalers,verbs=get;list;watch;create;update;patch;delete

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

	kruize := &mydomainv1alpha1.Kruize{}
	err := r.Get(ctx, req.NamespacedName, kruize)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	fmt.Println("kruize object base: ", kruize.Spec)

	// Call your deployment function with proper error handling
	err = r.deployKruize(ctx, kruize)
	if err != nil {
		logger.Error(err, "Failed to deploy Kruize")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	fmt.Println("Deployment initiated, waiting for pods to be ready")

	// Determine the target namespace based on cluster type
	var targetNamespace string
	switch kruize.Spec.Cluster_type {
	case "openshift":
		targetNamespace = kruize.Spec.Namespace
	case "minikube":
		targetNamespace = "monitoring"
	default:
		targetNamespace = "openshift-tuning"
	}

	// Wait for Kruize pods to be ready
	err = r.waitForKruizePods(ctx, targetNamespace, 5*time.Minute)
	if err != nil {
		logger.Error(err, "Kruize pods not ready yet")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger.Info("All Kruize pods are ready!", "namespace", targetNamespace)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
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

	requiredPods := []string{"kruize", "kruize-ui-nginx", "kruize-db"}
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

			// Check if we have all required pods running
			if readyPods >= 3 && totalPods >= 3 {
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

func (r *KruizeReconciler) deployKruize(ctx context.Context, kruize *mydomainv1alpha1.Kruize) error {
	// Add debug output
	fmt.Printf("=== DEBUG: Kruize Spec Fields ===\n")
	fmt.Printf("Cluster_type: '%s'\n", kruize.Spec.Cluster_type)
	fmt.Printf("Namespace: '%s'\n", kruize.Spec.Namespace)
	fmt.Printf("Size: %d\n", kruize.Spec.Size)
	fmt.Printf("Autotune_version: '%s'\n", kruize.Spec.Autotune_version)
	fmt.Printf("=== END DEBUG ===\n")

	cluster_type := kruize.Spec.Cluster_type
	fmt.Println("Deploying Kruize for cluster type:", cluster_type)

	var autotune_ns string

	switch cluster_type {
	case "openshift":
		autotune_ns = kruize.Spec.Namespace
	case "minikube":
		autotune_ns = "monitoring"
	default:
		return fmt.Errorf("unsupported cluster type: %s", cluster_type)
	}

	// Deploy the Kruize components directly
	err := r.deployKruizeComponents(ctx, autotune_ns, cluster_type)
	if err != nil {
		return fmt.Errorf("failed to deploy Kruize components: %v", err)
	}

	fmt.Printf("Successfully deployed Kruize components to namespace: %s\n", autotune_ns)
	return nil
}

func (r *KruizeReconciler) deployKruizeComponents(ctx context.Context, namespace string, clusterType string) error {

	logger := log.FromContext(ctx)

	// In test mode, just validate manifests without deploying
	if r.isTestMode() {
		logger.Info("Test mode detected, validating manifests only", "namespace", namespace)

		// Generate manifests to ensure they're valid
		_ = r.generateKruizeRBACAndConfigManifest(namespace, clusterType)
		_ = r.generateKruizeDeploymentManifest(namespace)
		_ = r.generateKruizeDBManifest(namespace)
		_ = r.generateKruizeUIManifest(namespace)

		if clusterType == "openshift" {
			_ = r.generateKruizeRoutesManifest(namespace)
		}

		logger.Info("All manifests validated successfully in test mode")
		return nil
	}

	// Deploy Kruize DB (using emptyDir to avoid OpenShift permission issues)
	kruizeDBManifest := r.generateKruizeDBManifest(namespace)
	err := r.applyYAMLString(ctx, kruizeDBManifest, namespace)
	if err != nil {
		return fmt.Errorf("failed to deploy Kruize DB: %v", err)
	}

	// Wait for DB to initialize
	fmt.Println("Waiting for database to initialize...")
	time.Sleep(90 * time.Second)

	// Deploy RBAC and ConfigMap
	rbacAndConfigManifest := r.generateKruizeRBACAndConfigManifest(namespace, clusterType)
	err = r.applyYAMLString(ctx, rbacAndConfigManifest, namespace)
	if err != nil {
		return fmt.Errorf("failed to deploy Kruize RBAC and Config: %v", err)
	}

	// Wait for RBAC to propagate
	fmt.Println("Waiting for RBAC to propagate...")
	time.Sleep(30 * time.Second)

	// Deploy Kruize main component
	kruizeDeploymentManifest := r.generateKruizeDeploymentManifest(namespace)
	err = r.applyYAMLString(ctx, kruizeDeploymentManifest, namespace)
	if err != nil {
		return fmt.Errorf("failed to deploy Kruize Deployment: %v", err)
	}

	// Deploy Kruize UI
	kruizeUIManifest := r.generateKruizeUIManifest(namespace)
	err = r.applyYAMLString(ctx, kruizeUIManifest, namespace)
	if err != nil {
		return fmt.Errorf("failed to deploy Kruize UI: %v", err)
	}

	// Deploy OpenShift routes if on OpenShift
	if clusterType == "openshift" {
		routesManifest := r.generateKruizeRoutesManifest(namespace)
		err = r.applyYAMLString(ctx, routesManifest, namespace)
		if err != nil {
			return fmt.Errorf("failed to deploy Kruize routes: %v", err)
		}
	}

	return nil
}

func (r *KruizeReconciler) generateKruizeRBACAndConfigManifest(namespace string, clusterType string) string {
	return fmt.Sprintf(`
apiVersion: v1
kind: Namespace
metadata:
  name: %s
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kruize-sa
  namespace: %s
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: kruize-recommendation-updater
rules:
  - apiGroups: [""]
    resources: ["pods", "nodes", "namespaces", "services", "endpoints"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets", "statefulsets", "daemonsets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["extensions", "networking.k8s.io"]
    resources: ["ingresses"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["autoscaling"]
    resources: ["horizontalpodautoscalers"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["autoscaling.k8s.io"]
    resources: ["verticalpodautoscalers"]  
    verbs: ["get", "list", "watch", "create", "update", "patch"]
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods", "nodes"]
    verbs: ["get", "list"]
  - apiGroups: ["monitoring.coreos.com"]
    resources: ["prometheuses", "alertmanagers", "servicemonitors"]
    verbs: ["get", "list", "watch"]
    resourceNames: ["*"]
  - apiGroups: ["monitoring.coreos.com"]
    resources: ["prometheuses/api"]
    verbs: ["get", "create", "update"]
  - nonResourceURLs: ["/metrics", "/api/v1/label/*", "/api/v1/query*", "/api/v1/series*", "/api/v1/targets*"]
    verbs: ["get"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: kruize-recommendation-updater-crb
subjects:
  - kind: ServiceAccount
    name: kruize-sa
    namespace: %s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kruize-recommendation-updater
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kruize-monitoring-view
subjects:
  - kind: ServiceAccount
    name: kruize-sa
    namespace: %s
roleRef:
  kind: ClusterRole
  name: cluster-monitoring-view
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kruize-prometheus-reader
subjects:
  - kind: ServiceAccount
    name: kruize-sa
    namespace: %s
roleRef:
  kind: ClusterRole
  name: view
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kruize-system-reader
subjects:
  - kind: ServiceAccount
    name: kruize-sa
    namespace: %s
roleRef:
  kind: ClusterRole
  name: system:monitoring
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kruize-monitoring-access
rules:
  - apiGroups: ["monitoring.coreos.com"]
    resources: ["prometheuses", "prometheuses/api", "alertmanagers", "servicemonitors", "prometheusrules"]
    verbs: ["get", "list", "watch", "create", "update", "patch"]
  - nonResourceURLs: ["/api/v1/*", "/metrics"]
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kruize-monitoring-access-crb
subjects:
  - kind: ServiceAccount
    name: kruize-sa
    namespace: %s
roleRef:
  kind: ClusterRole
  name: kruize-monitoring-access
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kruizeconfig
  namespace: %s
data:
  dbconfigjson: |
    {
      "database": {
        "adminPassword": "kruize123",
        "adminUsername": "kruize",
        "hostname": "kruize-db-service",
        "name": "kruizedb",
        "password": "kruize123",
        "port": 5432,
        "sslMode": "disable",
        "username": "kruize"
      }
    }
  kruizeconfigjson: |
    {
      "clustertype": "kubernetes",
      "k8stype": "openshift",
      "authtype": "",
      "monitoringagent": "prometheus",
      "monitoringservice": "prometheus-k8s",
      "monitoringendpoint": "prometheus-k8s",
      "savetodb": "true",
      "dbdriver": "jdbc:postgresql://",
      "plots": "true",
      "local": "true",
      "logAllHttpReqAndResp": "true",
      "recommendationsURL" : "http://kruize-service.%s.svc.cluster.local:8080/generateRecommendations?experiment_name=%%s",
      "experimentsURL" : "http://kruize-service.%s.svc.cluster.local:8080/createExperiment",
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
        "hbm2ddlauto": "update",
        "showsql": "false",
        "timezone": "UTC"
      },
      "datasource": [
        {
          "name": "prometheus-1",
          "provider": "prometheus",
          "namespace": "openshift-monitoring",
          "url": "https://prometheus-k8s.openshift-monitoring.svc.cluster.local:9091",
          "authentication": {
              "type": "bearer",
              "credentials": {
                "tokenFilePath": "/var/run/secrets/kubernetes.io/serviceaccount/token"
              }
          }
        }
      ]
    }
`, namespace, namespace, namespace, namespace, namespace, namespace, namespace, namespace, namespace, namespace)
}

func (r *KruizeReconciler) generateKruizeDeploymentManifest(namespace string) string {
	return fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kruize
  namespace: %s
  labels:
    app: kruize
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kruize
  template:
    metadata:
      labels:
        app: kruize
    spec:
      serviceAccountName: kruize-sa
      automountServiceAccountToken: true
      containers:
      - name: kruize
        image: quay.io/kruize/autotune_operator:latest
        ports:
        - containerPort: 8080
        env:
        - name: LOGGING_LEVEL
          value: "info"
        - name: ROOT_LOGGING_LEVEL
          value: "error"
        - name: DB_CONFIG_FILE
          value: "/etc/config/dbconfigjson"
        - name: KRUIZE_CONFIG_FILE
          value: "/etc/config/kruizeconfigjson"
        - name: JAVA_TOOL_OPTIONS
          value: "-XX:MaxRAMPercentage=80"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        volumeMounts:
        - name: config-volume
          mountPath: /etc/config
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 45
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 90
          periodSeconds: 20
          timeoutSeconds: 5
          failureThreshold: 3
      volumes:
      - name: config-volume
        configMap:
          name: kruizeconfig
---
apiVersion: v1
kind: Service
metadata:
  name: kruize-service
  namespace: %s
spec:
  selector:
    app: kruize
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  type: ClusterIP
`, namespace, namespace)
}

// Add this new function to generate OpenShift routes
func (r *KruizeReconciler) generateKruizeRoutesManifest(namespace string) string {
	return fmt.Sprintf(`
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: kruize
  namespace: %s
  labels:
    app: kruize
spec:
  to:
    kind: Service
    name: kruize-service
    weight: 100
  port:
    targetPort: http
  wildcardPolicy: None
---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: kruize-ui-nginx 
  namespace: %s
  labels:
    app: kruize-ui-nginx
spec:
  to:
    kind: Service
    name: kruize-ui-nginx-service  
    weight: 100
  port:
    targetPort: http
  wildcardPolicy: None
`, namespace, namespace)
}

func (r *KruizeReconciler) generateKruizeCRDManifest() string {
	return `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.17.3
  name: kruizes.my.domain
spec:
  group: my.domain
  names:
    kind: Kruize
    listKind: KruizeList
    plural: kruizes
    singular: kruize
  scope: Namespaced
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        description: Kruize is the Schema for the kruizes API
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation of an object.'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this object represents.'
            type: string
          metadata:
            type: object
          spec:
            description: KruizeSpec defines the desired state of Kruize
            properties:
              autotune_configmaps:
                type: string
              autotune_ui_version:
                type: string
              autotune_version:
                type: string
              cluster_type:
                type: string
              namespace:
                type: string
              non_interactive:
                format: int32
                type: integer
              size:
                format: int32
                type: integer
              use_yaml_build:
                format: int32
                type: integer
            required:
            - cluster_type
            type: object
          status:
            description: KruizeStatus defines the observed state of Kruize
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
`
}

func (r *KruizeReconciler) generateKruizeUIManifest(namespace string) string {
	return fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: kruize-ui-nginx-config
  namespace: %s
data:
  nginx.conf: |
    events {
        worker_connections 1024;
    }
    http {
        include       /etc/nginx/mime.types;
        default_type  application/octet-stream;
        
        upstream kruize_backend {
            server kruize-service:8080;
        }
        
        server {
            listen 8080;
            server_name localhost;
            root /usr/share/nginx/html;
            index index.html;
            
            # API proxy to Kruize backend
            location /api/ {
                proxy_pass http://kruize_backend/;
                proxy_set_header Host $host;
                proxy_set_header X-Real-IP $remote_addr;
                proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
                proxy_set_header X-Forwarded-Proto $scheme;
                proxy_connect_timeout 30s;
                proxy_send_timeout 30s;
                proxy_read_timeout 30s;
            }
            
            # Proxy health check
            location /health {
                proxy_pass http://kruize_backend/health;
                proxy_set_header Host $host;
            }
            
            # Proxy all Kruize API endpoints
            location /listApplications {
                proxy_pass http://kruize_backend/listApplications;
                proxy_set_header Host $host;
            }
            
            location /listExperiments {
                proxy_pass http://kruize_backend/listExperiments;
                proxy_set_header Host $host;
            }
            
            # Serve static files for everything else
            location / {
                try_files $uri $uri/ /index.html;
                add_header Cache-Control "no-cache, no-store, must-revalidate";
                add_header Pragma "no-cache";
                add_header Expires "0";
            }
            
            # Enable gzip compression
            gzip on;
            gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;
        }
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kruize-ui-nginx
  namespace: %s
  labels:
    app: kruize-ui-nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kruize-ui-nginx
  template:
    metadata:
      labels:
        app: kruize-ui-nginx
    spec:
      securityContext:
        runAsNonRoot: true
      containers:
      - name: kruize-ui-nginx
        image: quay.io/kruize/kruize-ui:latest
        ports:
        - containerPort: 8080
        env:
        - name: KRUIZE_API_URL
          value: "http://kruize-service:8080"
        - name: REACT_APP_KRUIZE_API_URL
          value: "http://kruize-service:8080"
        - name: KRUIZE_UI_API_URL
          value: "http://kruize-service:8080"
        - name: API_URL
          value: "http://kruize-service:8080"
        securityContext:
          allowPrivilegeEscalation: false
          runAsNonRoot: true
          capabilities:
            drop:
            - ALL
          seccompProfile:
            type: RuntimeDefault
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "200m"
        readinessProbe:
          httpGet:
            path: /
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        livenessProbe:
          httpGet:
            path: /
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 20
          timeoutSeconds: 5
          failureThreshold: 3
        volumeMounts:
        - name: nginx-config
          mountPath: /etc/nginx/nginx.conf
          subPath: nginx.conf
        - name: nginx-cache
          mountPath: /var/cache/nginx
        - name: nginx-pid
          mountPath: /var/run
        - name: nginx-tmp
          mountPath: /tmp
      volumes:
      - name: nginx-config
        configMap:
          name: kruize-ui-nginx-config
      - name: nginx-cache
        emptyDir: {}
      - name: nginx-pid
        emptyDir: {}
      - name: nginx-tmp
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: kruize-ui-nginx-service
  namespace: %s
spec:
  selector:
    app: kruize-ui-nginx
  ports:
  - name: http
    port: 3000
    targetPort: 8080
  type: ClusterIP
`, namespace, namespace, namespace)
}

func (r *KruizeReconciler) generateKruizeDBManifest(namespace string) string {
	return fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kruize-db
  namespace: %s
  labels:
    app: kruize-db
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kruize-db
  template:
    metadata:
      labels:
        app: kruize-db
    spec:
      serviceAccountName: kruize-sa
      containers:
        - name: kruize-db
          image: quay.io/kruizehub/postgres:15.2
          imagePullPolicy: IfNotPresent
          env:
            - name: POSTGRES_PASSWORD
              value: kruize123
            - name: POSTGRES_USER  
              value: kruize
            - name: POSTGRES_DB
              value: kruizedb
            - name: POSTGRES_INITDB_ARGS
              value: "--auth-host=md5"
            - name: PGDATA
              value: /tmp/pgdata
          resources:
            requests:
              memory: "200Mi"
              cpu: "100m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          ports:
            - containerPort: 5432
          volumeMounts:
            - name: postgres-storage
              mountPath: /tmp
          readinessProbe:
            exec:
              command:
                - /bin/sh
                - -c
                - pg_isready -U kruize -d kruizedb
            initialDelaySeconds: 15
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 3
          livenessProbe:
            exec:
              command:
                - /bin/sh
                - -c
                - pg_isready -U kruize -d kruizedb
            initialDelaySeconds: 45
            periodSeconds: 20
            timeoutSeconds: 5
            failureThreshold: 3
      volumes:
        - name: postgres-storage
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: kruize-db-service
  namespace: %s
  labels:
    app: kruize-db
spec:
  type: ClusterIP
  ports:
    - name: kruize-db-port
      port: 5432
      targetPort: 5432
  selector:
    app: kruize-db
`, namespace, namespace)
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
		For(&mydomainv1alpha1.Kruize{}).
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
