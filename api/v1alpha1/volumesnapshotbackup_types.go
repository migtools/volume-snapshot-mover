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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VolumeSnapshotBackupSpec defines the desired state of VolumeSnapshotBackup
type VolumeSnapshotBackupSpec struct {
	VolumeSnapshotContent corev1.ObjectReference `json:"volumeSnapshotContent,omitempty"`
	// Namespace where the Velero deployment is present
	ProtectedNamespace string `json:"protectedNamespace,omitempty"`
	// Restic Secret reference for given BSL
	ResticSecretRef corev1.LocalObjectReference `json:"resticSecretRef,omitempty"`
	// StorageClassName can be used to override the StorageClass of the source PVC
	//+optional
	StorageClassName string `json:"storageClassName,omitempty"`
	// AccessMode can be used to override the accessMode of the source PVC
	//+optional
	AccessMode []corev1.PersistentVolumeAccessMode `json:"accessMode,omitempty"`
	// cacheStorageClassName is the storageClass that should be used when provisioning
	// the cache volume
	//+optional
	CacheStorageClassName string `json:"cacheStorageClassName,omitempty"`
	// cacheCapacity determines the size of the restic metadata cache volume
	//+optional
	CacheCapacity *resource.Quantity `json:"cacheCapacity,omitempty"`
	// cacheAccessMode is the access mode to be used to provision the cache volume
	//+optional
	CacheAccessMode []corev1.PersistentVolumeAccessMode `json:"cacheAccessMode,omitempty"`
}

// VolumeSnapshotBackupStatus defines the observed state of VolumeSnapshotBackup
type VolumeSnapshotBackupStatus struct {
	Completed bool `json:"completed,omitempty"`
	// Include references to the volsync CRs and their state as they are
	// running
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Includes source PVC name and size
	SourcePVCData PVCData `json:"sourcePVCData,omitempty"`
	// Includes restic repository path
	ResticRepository string `json:"resticrepository,omitempty"`
	// volumesnapshot backup phase status
	Phase VolumeSnapshotBackupPhase `json:"phase,omitempty"`
	// name of the VolumeSnapshotClass
	VolumeSnapshotClassName string `json:"volumeSnapshotClassName,omitempty"`
	// StartTimestamp records the time a volsumesnapshotbackup was started.
	// +optional
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`
	// CompletionTimestamp records the time a volumesnapshotbackup reached a terminal state.
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`
	// Includes information pertaining to Volsync ReplicationSource CR
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
	// name of the source PersistentVolumeClaim
	Name string `json:"name,omitempty"`
	// size of the PersistentVolumeClaim
	Size string `json:"size,omitempty"`
	// name of the StorageClass of the source PVC
	StorageClassName string `json:"storageClassName,omitempty"`
	// accessMode of the source PVC
	AccessMode []corev1.PersistentVolumeAccessMode `json:"accessMode,omitempty"`
}

type VolumeSnapshotBackupPhase string

const (
	SnapMoverVolSyncPhaseCompleted VolumeSnapshotBackupPhase = "SnapshotBackupDone"

	SnapMoverBackupPhaseCompleted VolumeSnapshotBackupPhase = "Completed"

	SnapMoverBackupPhaseInProgress VolumeSnapshotBackupPhase = "InProgress"

	SnapMoverBackupPhaseFailed VolumeSnapshotBackupPhase = "Failed"

	SnapMoverBackupPhasePartiallyFailed VolumeSnapshotBackupPhase = "PartiallyFailed"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=volumesnapshotbackups,shortName=vsb

// VolumeSnapshotBackup is the Schema for the volumesnapshotbackups API
type VolumeSnapshotBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VolumeSnapshotBackupSpec   `json:"spec,omitempty"`
	Status VolumeSnapshotBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VolumeSnapshotBackupList contains a list of VolumeSnapshotBackup
type VolumeSnapshotBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeSnapshotBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VolumeSnapshotBackup{}, &VolumeSnapshotBackupList{})
}
