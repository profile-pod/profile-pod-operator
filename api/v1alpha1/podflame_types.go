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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PodFlameSpec defines the desired state of PodFlame
type PodFlameSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	TargetPod string `json:"targetPod,omitempty"`

	// +kubebuilder:validation:Enum:="cpu"
	// +kubebuilder:validation:Required
	// +kubebuilder:default:=cpu
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Event string `json:"event,omitempty"`

	// +kubebuilder:default:="2m"
	// +kubebuilder:validation:Pattern:="^(([1-6]{0,1}[0-9])([mM]{1}))?(([1-6]{0,1}[0-9])([sS]{1}))?$"
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Duration string `json:"duration,omitempty"`

	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ContainerName string `json:"containerName,omitempty"`
}

// PodFlameStatus defines the observed state of PodFlame
type PodFlameStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	FlameGraph string `json:"flameGraph,omitempty"`

	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Failed string `json:"failed,omitempty" protobuf:"varint,6,opt,name=failed"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:shortName="pf"

// PodFlame is the Schema for the podflames API
type PodFlame struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodFlameSpec   `json:"spec,omitempty"`
	Status PodFlameStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PodFlameList contains a list of PodFlame
type PodFlameList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodFlame `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodFlame{}, &PodFlameList{})
}
