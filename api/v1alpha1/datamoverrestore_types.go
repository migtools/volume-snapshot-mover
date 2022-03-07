/*
Copyright 2022.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DataMoverRestoreSpec defines the desired state of DataMoverRestore
type DataMoverRestoreSpec struct {
	ResticSecretRef     corev1.LocalObjectReference `json:"resticSecretRef,omitempty"`
	DestinationClaimRef corev1.ObjectReference      `json:"destinationClaimRef,omitempty"`
}

// DataMoverRestoreStatus defines the observed state of DataMoverRestore
type DataMoverRestoreStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	Completed  bool               `json:"completed"`
	// TODO: Add data mover refs and velero refs
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DataMoverRestore is the Schema for the datamoverrestores API
type DataMoverRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DataMoverRestoreSpec   `json:"spec,omitempty"`
	Status DataMoverRestoreStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DataMoverRestoreList contains a list of DataMoverRestore
type DataMoverRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DataMoverRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DataMoverRestore{}, &DataMoverRestoreList{})
}
