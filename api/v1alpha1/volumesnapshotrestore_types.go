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

// VolumeSnapshotRestoreSpec defines the desired state of VolumeSnapshotRestore
type VolumeSnapshotRestoreSpec struct {
	ResticSecretRef corev1.LocalObjectReference `json:"resticSecretRef,omitempty"`
	// Includes associated volumesnapshotbackup details
	DataMoverBackupref DMBRef `json:"dataMoverBackupRef,omitempty"`
	// Namespace where the Velero deployment is present
	ProtectedNamespace string `json:"protectedNamespace,omitempty"`
}

// VolumeSnapshotRestoreStatus defines the observed state of VolumeSnapshotRestore
type VolumeSnapshotRestoreStatus struct {
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
	Completed      bool               `json:"completed"`
	SnapshotHandle *string            `json:"snapshotHandle,omitempty"`
}

type DMBRef struct {
	// Includes backed up PVC name and size
	BackedUpPVCData PVCData `json:"sourcePVCData,omitempty"`
	// Includes restic repository path
	ResticRepository string `json:"resticrepository,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VolumeSnapshotRestore is the Schema for the volumesnapshotrestores API
type VolumeSnapshotRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VolumeSnapshotRestoreSpec   `json:"spec,omitempty"`
	Status VolumeSnapshotRestoreStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VolumeSnapshotRestoreList contains a list of VolumeSnapshotRestore
type VolumeSnapshotRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeSnapshotRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VolumeSnapshotRestore{}, &VolumeSnapshotRestoreList{})
}
