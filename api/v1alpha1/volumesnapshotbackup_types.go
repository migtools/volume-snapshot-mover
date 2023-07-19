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

// VolumeSnapshotBackupSpec defines the desired state of VolumeSnapshotBackup
type VolumeSnapshotBackupSpec struct {
	VolumeSnapshotContent corev1.ObjectReference `json:"volumeSnapshotContent,omitempty"`
	// Namespace where the Velero deployment is present
	ProtectedNamespace string `json:"protectedNamespace,omitempty"`
	// Restic Secret reference for given BSL
	ResticSecretRef corev1.LocalObjectReference `json:"resticSecretRef,omitempty"` // override description to avoid showing TODO? https://pkg.go.dev/k8s.io/api/core/v1#LocalObjectReference
}

// VolumeSnapshotBackupStatus defines the observed state of VolumeSnapshotBackup
type VolumeSnapshotBackupStatus struct {
	Completed bool `json:"completed,omitempty"`
	// Include references to the VolSync CRs and their states as they are
	// running
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Includes source PVC name and size
	SourcePVCData PVCData `json:"sourcePVCData,omitempty"`
	// Includes restic repository path
	ResticRepository string `json:"resticrepository,omitempty"`
	// volumesnapshot backup phase status
	// +kubebuilder:validation:Enum=SnapshotBackupDone;Completed;InProgress;Failed;PartiallyFailed;Cleanup
	Phase VolumeSnapshotBackupPhase `json:"phase,omitempty"`
	// volumesnapshotbackup batching status
	// +kubebuilder:validation:Enum=Completed;Queued;Processing
	BatchingStatus VolumeSnapshotBackupBatchingStatus `json:"batchingStatus,omitempty"`
	// name of the VolumeSnapshotClass
	VolumeSnapshotClassName string `json:"volumeSnapshotClassName,omitempty"`
	// StartTimestamp records the time a volsumesnapshotbackup was started.
	// +optional
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`
	// CompletionTimestamp records the time a volumesnapshotbackup reached a terminal state.
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`
	// Includes information pertaining to VolSync ReplicationSource CR
	ReplicationSourceData ReplicationSourceData `json:"replicationSourceData,omitempty"`
}

type ReplicationSourceData struct {
	// name of the ReplicationSource associated with the volumesnapshotbackup
	Name string `json:"name,omitempty"`
	// StartTimestamp records the time a ReplicationSource was started.
	// +optional
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`
	// CompletionTimestamp records the time a ReplicationSource reached a terminal state.
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`
}

type PVCData struct {
	// name of the PersistentVolumeClaim
	Name string `json:"name,omitempty"`
	// size of the PersistentVolumeClaim
	Size string `json:"size,omitempty"`
	// name of the StorageClass
	StorageClassName string `json:"storageClassName,omitempty"`
}

type VolumeSnapshotBackupPhase string

const (
	SnapMoverVolSyncPhaseCompleted VolumeSnapshotBackupPhase = "SnapshotBackupDone"

	SnapMoverBackupPhaseCompleted VolumeSnapshotBackupPhase = "Completed"

	SnapMoverBackupPhaseInProgress VolumeSnapshotBackupPhase = "InProgress"

	SnapMoverBackupPhaseFailed VolumeSnapshotBackupPhase = "Failed"

	SnapMoverBackupPhasePartiallyFailed VolumeSnapshotBackupPhase = "PartiallyFailed"

	SnapMoverBackupPhaseCleanup VolumeSnapshotBackupPhase = "Cleanup"
)

type VolumeSnapshotBackupBatchingStatus string

const (
	SnapMoverBackupBatchingCompleted VolumeSnapshotBackupBatchingStatus = "Completed"

	SnapMoverBackupBatchingQueued VolumeSnapshotBackupBatchingStatus = "Queued"

	SnapMoverBackupBatchingProcessing VolumeSnapshotBackupBatchingStatus = "Processing"
)

// VolumeSnapshotBackup is the Schema for the volumesnapshotbackups API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=volumesnapshotbackups,shortName=vsb
// +kubebuilder:printcolumn:name="PVC Name",type=string,JSONPath=".status.sourcePVCData.name"
// +kubebuilder:printcolumn:name="VolumeSnapshotContent",type=string,JSONPath=".spec.volumeSnapshotContent.name"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="BatchingStatus",type=string,JSONPath=".status.batchingStatus"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
type VolumeSnapshotBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VolumeSnapshotBackupSpec   `json:"spec,omitempty"`
	Status VolumeSnapshotBackupStatus `json:"status,omitempty"`
}

// VolumeSnapshotBackupList contains a list of VolumeSnapshotBackup
// +kubebuilder:object:root=true
type VolumeSnapshotBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeSnapshotBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VolumeSnapshotBackup{}, &VolumeSnapshotBackupList{})
}
