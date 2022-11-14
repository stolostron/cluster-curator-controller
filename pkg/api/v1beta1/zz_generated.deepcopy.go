//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*


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

package v1beta1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterCurator) DeepCopyInto(out *ClusterCurator) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterCurator.
func (in *ClusterCurator) DeepCopy() *ClusterCurator {
	if in == nil {
		return nil
	}
	out := new(ClusterCurator)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterCurator) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterCuratorList) DeepCopyInto(out *ClusterCuratorList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterCurator, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterCuratorList.
func (in *ClusterCuratorList) DeepCopy() *ClusterCuratorList {
	if in == nil {
		return nil
	}
	out := new(ClusterCuratorList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterCuratorList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterCuratorSpec) DeepCopyInto(out *ClusterCuratorSpec) {
	*out = *in
	in.Install.DeepCopyInto(&out.Install)
	in.Scale.DeepCopyInto(&out.Scale)
	in.Destroy.DeepCopyInto(&out.Destroy)
	in.Upgrade.DeepCopyInto(&out.Upgrade)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterCuratorSpec.
func (in *ClusterCuratorSpec) DeepCopy() *ClusterCuratorSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterCuratorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterCuratorStatus) DeepCopyInto(out *ClusterCuratorStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterCuratorStatus.
func (in *ClusterCuratorStatus) DeepCopy() *ClusterCuratorStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterCuratorStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Hook) DeepCopyInto(out *Hook) {
	*out = *in
	if in.ExtraVars != nil {
		in, out := &in.ExtraVars, &out.ExtraVars
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Hook.
func (in *Hook) DeepCopy() *Hook {
	if in == nil {
		return nil
	}
	out := new(Hook)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Hooks) DeepCopyInto(out *Hooks) {
	*out = *in
	if in.Prehook != nil {
		in, out := &in.Prehook, &out.Prehook
		*out = make([]Hook, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Posthook != nil {
		in, out := &in.Posthook, &out.Posthook
		*out = make([]Hook, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.OverrideJob != nil {
		in, out := &in.OverrideJob, &out.OverrideJob
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Hooks.
func (in *Hooks) DeepCopy() *Hooks {
	if in == nil {
		return nil
	}
	out := new(Hooks)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpgradeHooks) DeepCopyInto(out *UpgradeHooks) {
	*out = *in
	if in.Prehook != nil {
		in, out := &in.Prehook, &out.Prehook
		*out = make([]Hook, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Posthook != nil {
		in, out := &in.Posthook, &out.Posthook
		*out = make([]Hook, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.OverrideJob != nil {
		in, out := &in.OverrideJob, &out.OverrideJob
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpgradeHooks.
func (in *UpgradeHooks) DeepCopy() *UpgradeHooks {
	if in == nil {
		return nil
	}
	out := new(UpgradeHooks)
	in.DeepCopyInto(out)
	return out
}
