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

// DataMoverBackupSpec defines the desired state of DataMoverBackup
type DataMoverBackupSpec struct {
	VolumeSnapshotContent corev1.ObjectReference `json:"volumeSnapshotContent,omitempty"`
}

// DataMoverBackupStatus defines the observed state of DataMoverBackup
type DataMoverBackupStatus struct {
	Completed bool `json:"completed,omitempty"`
	// Include references to the volsync CRs and their state as they are
	// running
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Includes source PVC name and size
	SourcePVCData PVCData `json:"sourcePVCData,omitempty"`
	// Includes restic repository path
	ResticRepository string `json:"resticrepository,omitempty"`
	// Datamovement backup phase status
	Phase DatamoverBackupPhase `json:"phase,omitempty"`
}

type PVCData struct {
	// name of the PersistentVolumeClaim
	Name string `json:"name, omitempty"`
	// size of the PersistentVolumeClaim
	Size int32 `json:"size, omitempty"`
}

type DatamoverBackupPhase string

const (
	DatamoverBackupPhaseCompleted DatamoverBackupPhase = "Completed"

	DatamoverBackupPhaseInProgress DatamoverBackupPhase = "InProgress"

	DatamoverBackupPhaseFailed DatamoverBackupPhase = "Failed"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DataMoverBackup is the Schema for the datamoverbackups API
type DataMoverBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DataMoverBackupSpec   `json:"spec,omitempty"`
	Status DataMoverBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DataMoverBackupList contains a list of DataMoverBackup
type DataMoverBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DataMoverBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DataMoverBackup{}, &DataMoverBackupList{})
}
