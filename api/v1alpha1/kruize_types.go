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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KruizeSpec defines the desired state of Kruize
type KruizeSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Type of Kubernetes cluster (openshift, minikube, or kind)
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Cluster Type",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:select:openshift","urn:alm:descriptor:com.tectonic.ui:select:minikube","urn:alm:descriptor:com.tectonic.ui:select:kind"}
	Cluster_type      string `json:"cluster_type"`

	// Container image for Kruize Autotune
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Autotune Image",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Autotune_image    string `json:"autotune_image"`

	// Container image for Kruize UI
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Autotune UI Image",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Autotune_ui_image string `json:"autotune_ui_image"`

	// Container image for Kruize Optimizer
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Optimizer Image",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Optimizer_image   string `json:"optimizer_image,omitempty"`

	// Target namespace for Kruize deployment
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Namespace",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Namespace         string `json:"namespace"`

	// Persistent Volume configuration
	// +optional
	PersistentVolume *PersistentVolumeSpec `json:"persistentVolume,omitempty"`

	// Persistent Volume Claim configuration
	// +optional
	PersistentVolumeClaim *PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`

	// Database resource configuration
	// +optional
	KruizeDB *KruizeDBConfig `json:"kruize-db,omitempty"`

	// Kruize application resource configuration
	// +optional
	Kruize *KruizeAppConfig `json:"kruize,omitempty"`
}

// KubernetesResourceRequirements defines Kubernetes-style resource requirements
type KubernetesResourceRequirements struct {
	// Resource requests
	// +optional
	Requests *ResourceList `json:"requests,omitempty"`

	// Resource limits
	// +optional
	Limits *ResourceList `json:"limits,omitempty"`
}

// ResourceList defines CPU and memory resources
type ResourceList struct {
	// Memory (e.g., "100Mi", "1Gi")
	// +optional
	Memory string `json:"memory,omitempty"`

	// CPU (e.g., "0.5", "500m")
	// +optional
	CPU string `json:"cpu,omitempty"`
}

// KruizeAppConfig defines configuration for Kruize application
type KruizeAppConfig struct {
	// Resource requirements
	// +optional
	Resources *KubernetesResourceRequirements `json:"resources,omitempty"`
}

// KruizeDBConfig defines configuration for Kruize database
type KruizeDBConfig struct {
	// Resource requirements
	// +optional
	Resources *KubernetesResourceRequirements `json:"resources,omitempty"`

	// Volume mounts for the container
	// +optional
	VolumeMounts []VolumeMount `json:"volumeMounts,omitempty"`

	// Volumes for the pod
	// +optional
	Volumes []Volume `json:"volumes,omitempty"`
}

// VolumeMount describes a mounting of a Volume within a container
type VolumeMount struct {
	// Name of the volume to mount
	Name string `json:"name"`

	// Path within the container at which the volume should be mounted
	MountPath string `json:"mountPath"`
}

// Volume represents a named volume in a pod
type Volume struct {
	// Name of the volume
	Name string `json:"name"`

	// PersistentVolumeClaim represents a reference to a PersistentVolumeClaim
	// +optional
	PersistentVolumeClaim *PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

// PersistentVolumeClaimVolumeSource references a PVC in the same namespace
type PersistentVolumeClaimVolumeSource struct {
	// ClaimName is the name of the PVC in the same namespace
	ClaimName string `json:"claimName"`
}

// PersistentVolumeAccessMode defines the access mode for persistent volumes
// +kubebuilder:validation:Enum=ReadWriteOnce;ReadOnlyMany;ReadWriteMany;ReadWriteOncePod
type PersistentVolumeAccessMode string

const (
	// ReadWriteOnce allows read-write access by a single node
	ReadWriteOnce PersistentVolumeAccessMode = "ReadWriteOnce"
	// ReadOnlyMany allows read-only access by multiple nodes
	ReadOnlyMany PersistentVolumeAccessMode = "ReadOnlyMany"
	// ReadWriteMany allows read-write access by multiple nodes
	ReadWriteMany PersistentVolumeAccessMode = "ReadWriteMany"
	// ReadWriteOncePod allows read-write access by a single pod
	ReadWriteOncePod PersistentVolumeAccessMode = "ReadWriteOncePod"
)

// PersistentVolumeSpec defines PersistentVolume configuration
type PersistentVolumeSpec struct {
	// Name of the PersistentVolume
	// +optional
	Name string `json:"name,omitempty"`

	// Storage class name
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// Capacity defines the storage capacity
	// +optional
	Capacity *StorageCapacity `json:"capacity,omitempty"`

	// Access modes for the persistent volume
	// +optional
	// +kubebuilder:validation:MaxItems=4
	AccessModes []PersistentVolumeAccessMode `json:"accessModes,omitempty"`

	// Host path configuration
	// +optional
	HostPath *HostPathVolumeSource `json:"hostPath,omitempty"`

	// Labels for the PersistentVolume
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// PersistentVolumeClaimSpec defines PersistentVolumeClaim configuration
type PersistentVolumeClaimSpec struct {
	// Name of the PersistentVolumeClaim
	// +optional
	Name string `json:"name,omitempty"`

	// Storage class name
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// Access modes for the persistent volume claim
	// +optional
	// +kubebuilder:validation:MaxItems=4
	AccessModes []PersistentVolumeAccessMode `json:"accessModes,omitempty"`

	// Resources defines the storage resources
	// +optional
	Resources *PVCResourceRequirements `json:"resources,omitempty"`

	// Labels for the PersistentVolumeClaim
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// StorageCapacity defines storage capacity
type StorageCapacity struct {
	// Storage size (e.g., "500Mi", "1Gi")
	// +optional
	Storage string `json:"storage,omitempty"`
}

// HostPathVolumeSource represents a host path mapped into a pod
type HostPathVolumeSource struct {
	// Path of the directory on the host
	Path string `json:"path"`
}

// PVCResourceRequirements describes the storage resources required by a PVC
type PVCResourceRequirements struct {
	// Requests describes the minimum storage resources required
	// +optional
	Requests *StorageCapacity `json:"requests,omitempty"`
}

// KruizeStatus defines the observed state of Kruize
type KruizeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Nodes []string `json:"nodes"`
}

// Kruize contains configuration options for controlling the deployment of the Kruize
// application and its related components. A Kruize instance must be created to instruct
// the operator to deploy the Kruize application.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:resources={{Deployment,v1},{Service,v1},{ServiceAccount,v1},{ConfigMap,v1},{PersistentVolume,v1},{PersistentVolumeClaim,v1},{StorageClass,v1}}
type Kruize struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KruizeSpec   `json:"spec,omitempty"`
	Status KruizeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KruizeList contains a list of Kruize
type KruizeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kruize `json:"items"`
}

const (
    KruizeFinalizer = "kruize.io/finalizer"
)

func init() {
	SchemeBuilder.Register(&Kruize{}, &KruizeList{})
}
