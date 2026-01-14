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

	// Foo is an example field of Kruize. Edit kruize_types.go to remove/update
	Size                int32  `json:"size"`
	Cluster_type        string `json:"cluster_type"`
	Autotune_image      string `json:"autotune_image"`
	Autotune_ui_image   string `json:"autotune_ui_image"`
	Optimizer_image     string `json:"optimizer_image,omitempty"`
	Autotune_configmaps string `json:"autotune_configmaps"`
	Non_interactive     int32  `json:"non_interactive"`
	Use_yaml_build      int32  `json:"use_yaml_build"`
	Namespace           string `json:"namespace"`
}

// KruizeStatus defines the observed state of Kruize
type KruizeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Nodes []string `json:"nodes"`
}

// Kruize is the Schema for the kruizes API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
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

func init() {
	SchemeBuilder.Register(&Kruize{}, &KruizeList{})
}
