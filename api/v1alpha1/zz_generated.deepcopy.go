//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PVCData) DeepCopyInto(out *PVCData) {
	*out = *in
	if in.AccessMode != nil {
		in, out := &in.AccessMode, &out.AccessMode
		*out = make([]v1.PersistentVolumeAccessMode, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PVCData.
func (in *PVCData) DeepCopy() *PVCData {
	if in == nil {
		return nil
	}
	out := new(PVCData)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReplicationDestinationData) DeepCopyInto(out *ReplicationDestinationData) {
	*out = *in
	if in.StartTimestamp != nil {
		in, out := &in.StartTimestamp, &out.StartTimestamp
		*out = (*in).DeepCopy()
	}
	if in.CompletionTimestamp != nil {
		in, out := &in.CompletionTimestamp, &out.CompletionTimestamp
		*out = (*in).DeepCopy()
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReplicationDestinationData.
func (in *ReplicationDestinationData) DeepCopy() *ReplicationDestinationData {
	if in == nil {
		return nil
	}
	out := new(ReplicationDestinationData)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReplicationSourceData) DeepCopyInto(out *ReplicationSourceData) {
	*out = *in
	if in.StartTimestamp != nil {
		in, out := &in.StartTimestamp, &out.StartTimestamp
		*out = (*in).DeepCopy()
	}
	if in.CompletionTimestamp != nil {
		in, out := &in.CompletionTimestamp, &out.CompletionTimestamp
		*out = (*in).DeepCopy()
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReplicationSourceData.
func (in *ReplicationSourceData) DeepCopy() *ReplicationSourceData {
	if in == nil {
		return nil
	}
	out := new(ReplicationSourceData)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VSBRef) DeepCopyInto(out *VSBRef) {
	*out = *in
	in.BackedUpPVCData.DeepCopyInto(&out.BackedUpPVCData)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VSBRef.
func (in *VSBRef) DeepCopy() *VSBRef {
	if in == nil {
		return nil
	}
	out := new(VSBRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotBackup) DeepCopyInto(out *VolumeSnapshotBackup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotBackup.
func (in *VolumeSnapshotBackup) DeepCopy() *VolumeSnapshotBackup {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotBackup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeSnapshotBackup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotBackupList) DeepCopyInto(out *VolumeSnapshotBackupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VolumeSnapshotBackup, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotBackupList.
func (in *VolumeSnapshotBackupList) DeepCopy() *VolumeSnapshotBackupList {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotBackupList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeSnapshotBackupList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotBackupSpec) DeepCopyInto(out *VolumeSnapshotBackupSpec) {
	*out = *in
	out.VolumeSnapshotContent = in.VolumeSnapshotContent
	out.ResticSecretRef = in.ResticSecretRef
	if in.AccessMode != nil {
		in, out := &in.AccessMode, &out.AccessMode
		*out = make([]v1.PersistentVolumeAccessMode, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotBackupSpec.
func (in *VolumeSnapshotBackupSpec) DeepCopy() *VolumeSnapshotBackupSpec {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotBackupSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotBackupStatus) DeepCopyInto(out *VolumeSnapshotBackupStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.SourcePVCData.DeepCopyInto(&out.SourcePVCData)
	if in.StartTimestamp != nil {
		in, out := &in.StartTimestamp, &out.StartTimestamp
		*out = (*in).DeepCopy()
	}
	if in.CompletionTimestamp != nil {
		in, out := &in.CompletionTimestamp, &out.CompletionTimestamp
		*out = (*in).DeepCopy()
	}
	in.ReplicationSourceData.DeepCopyInto(&out.ReplicationSourceData)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotBackupStatus.
func (in *VolumeSnapshotBackupStatus) DeepCopy() *VolumeSnapshotBackupStatus {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotBackupStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotRestore) DeepCopyInto(out *VolumeSnapshotRestore) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotRestore.
func (in *VolumeSnapshotRestore) DeepCopy() *VolumeSnapshotRestore {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotRestore)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeSnapshotRestore) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotRestoreList) DeepCopyInto(out *VolumeSnapshotRestoreList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VolumeSnapshotRestore, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotRestoreList.
func (in *VolumeSnapshotRestoreList) DeepCopy() *VolumeSnapshotRestoreList {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotRestoreList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeSnapshotRestoreList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotRestoreSpec) DeepCopyInto(out *VolumeSnapshotRestoreSpec) {
	*out = *in
	out.ResticSecretRef = in.ResticSecretRef
	in.VolumeSnapshotMoverBackupref.DeepCopyInto(&out.VolumeSnapshotMoverBackupref)
	if in.AccessMode != nil {
		in, out := &in.AccessMode, &out.AccessMode
		*out = make([]v1.PersistentVolumeAccessMode, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotRestoreSpec.
func (in *VolumeSnapshotRestoreSpec) DeepCopy() *VolumeSnapshotRestoreSpec {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotRestoreSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeSnapshotRestoreStatus) DeepCopyInto(out *VolumeSnapshotRestoreStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.StartTimestamp != nil {
		in, out := &in.StartTimestamp, &out.StartTimestamp
		*out = (*in).DeepCopy()
	}
	if in.CompletionTimestamp != nil {
		in, out := &in.CompletionTimestamp, &out.CompletionTimestamp
		*out = (*in).DeepCopy()
	}
	in.ReplicationDestinationData.DeepCopyInto(&out.ReplicationDestinationData)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeSnapshotRestoreStatus.
func (in *VolumeSnapshotRestoreStatus) DeepCopy() *VolumeSnapshotRestoreStatus {
	if in == nil {
		return nil
	}
	out := new(VolumeSnapshotRestoreStatus)
	in.DeepCopyInto(out)
	return out
}
