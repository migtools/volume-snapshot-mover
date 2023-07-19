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
	VolumeSnapshotMoverBackupref VSBRef `json:"volumeSnapshotMoverBackupRef,omitempty"`
	// Namespace where the Velero deployment is present
	ProtectedNamespace string `json:"protectedNamespace,omitempty"`
}

// VolumeSnapshotRestoreStatus defines the observed state of VolumeSnapshotRestore
type VolumeSnapshotRestoreStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// volumesnapshot restore phase status
	// +kubebuilder:validation:Enum=SnapshotRestoreDone;Completed;InProgress;Failed;PartiallyFailed;Cleanup
	Phase VolumeSnapshotRestorePhase `json:"phase,omitempty"`
	// volumesnapshotrestore batching status
	// +kubebuilder:validation:Enum=Completed;Queued;Processing
	BatchingStatus VolumeSnapshotRestoreBatchingStatus `json:"batchingStatus,omitempty"`
	// name of the volumesnapshot snaphandle that is backed up
	SnapshotHandle string `json:"snapshotHandle,omitempty"`
	// name of the volumesnapshotcontent that is backed up
	VolumeSnapshotContentName string `json:"volumeSnapshotContentName,omitempty"`
	// StartTimestamp records the time a volsumesnapshotrestore was started.
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`
	// CompletionTimestamp records the time a volumesnapshotrestore reached a terminal state.
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`
	// Includes information pertaining to VolSync ReplicationDestination CR
	ReplicationDestinationData ReplicationDestinationData `json:"replicationDestinationData,omitempty"`
}

type VSBRef struct {
	// Includes backed up PVC name and size
	BackedUpPVCData PVCData `json:"sourcePVCData,omitempty"` // this is from here api/v1alpha1/volumesnapshotbackup_types.go line 72, how it got imported?
	// Includes restic repository path
	ResticRepository string `json:"resticrepository,omitempty"`
	// name of the VolumeSnapshotClass
	VolumeSnapshotClassName string `json:"volumeSnapshotClassName,omitempty"`
}

type ReplicationDestinationData struct {
	// name of the ReplicationDestination associated with the volumesnapshotrestore
	Name string `json:"name,omitempty"`
	// StartTimestamp records the time a ReplicationDestination was started.
	// +optional
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`
	// CompletionTimestamp records the time a ReplicationDestination reached a terminal state.
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`
}

// there is a way to extend types to avoid duplication?
type VolumeSnapshotRestorePhase string

const (
	SnapMoverRestoreVolSyncPhaseCompleted VolumeSnapshotRestorePhase = "SnapshotRestoreDone"

	SnapMoverRestorePhaseCompleted VolumeSnapshotRestorePhase = "Completed"

	SnapMoverRestorePhaseInProgress VolumeSnapshotRestorePhase = "InProgress"

	SnapMoverRestorePhaseFailed VolumeSnapshotRestorePhase = "Failed"

	SnapMoverRestorePhasePartiallyFailed VolumeSnapshotRestorePhase = "PartiallyFailed"

	SnapMoverRestorePhaseCleanup VolumeSnapshotRestorePhase = "Cleanup"
)

// Could we use the same type from backup?
type VolumeSnapshotRestoreBatchingStatus string

const (
	SnapMoverRestoreBatchingCompleted VolumeSnapshotRestoreBatchingStatus = "Completed"

	SnapMoverRestoreBatchingQueued VolumeSnapshotRestoreBatchingStatus = "Queued"

	SnapMoverRestoreBatchingProcessing VolumeSnapshotRestoreBatchingStatus = "Processing"
)

// VolumeSnapshotRestore is the Schema for the volumesnapshotrestores API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=volumesnapshotrestores,shortName=vsr
// +kubebuilder:printcolumn:name="PVC Name",type=string,JSONPath=".spec.volumeSnapshotMoverBackupRef.sourcePVCData.name"
// +kubebuilder:printcolumn:name="VolumeSnapshotContent",type=string,JSONPath=".status.volumeSnapshotContentName"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="BatchingStatus",type=string,JSONPath=".status.batchingStatus"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
type VolumeSnapshotRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VolumeSnapshotRestoreSpec   `json:"spec,omitempty"`
	Status VolumeSnapshotRestoreStatus `json:"status,omitempty"`
}

// VolumeSnapshotRestoreList contains a list of VolumeSnapshotRestore
// +kubebuilder:object:root=true
type VolumeSnapshotRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeSnapshotRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VolumeSnapshotRestore{}, &VolumeSnapshotRestoreList{})
}
