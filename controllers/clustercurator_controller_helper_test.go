// Copyright Contributors to the Open Cluster Management project.

package controllers

import (
	"testing"

	ccv1beta1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
)

func Test_containsWorkflowType(t *testing.T) {
	tests := []struct {
		name    string
		curator ccv1beta1.ClusterCurator
		want    bool
	}{
		{
			name: "no Workflow type",
			curator: ccv1beta1.ClusterCurator{
				Spec: ccv1beta1.ClusterCuratorSpec{},
			},
			want: false,
		},
		{
			name: "install prehook Workflow type",
			curator: ccv1beta1.ClusterCurator{
				Spec: ccv1beta1.ClusterCuratorSpec{
					Install: ccv1beta1.Hooks{
						Prehook: []ccv1beta1.Hook{{Type: ccv1beta1.HookTypeWorkflow}},
					},
				},
			},
			want: true,
		},
		{
			name: "install posthook Workflow type",
			curator: ccv1beta1.ClusterCurator{
				Spec: ccv1beta1.ClusterCuratorSpec{
					Install: ccv1beta1.Hooks{
						Posthook: []ccv1beta1.Hook{{Type: ccv1beta1.HookTypeWorkflow}},
					},
				},
			},
			want: true,
		},
		{
			name: "destroy prehook Workflow type",
			curator: ccv1beta1.ClusterCurator{
				Spec: ccv1beta1.ClusterCuratorSpec{
					Destroy: ccv1beta1.Hooks{
						Prehook: []ccv1beta1.Hook{{Type: ccv1beta1.HookTypeWorkflow}},
					},
				},
			},
			want: true,
		},
		{
			name: "destroy posthook Workflow type",
			curator: ccv1beta1.ClusterCurator{
				Spec: ccv1beta1.ClusterCuratorSpec{
					Destroy: ccv1beta1.Hooks{
						Posthook: []ccv1beta1.Hook{{Type: ccv1beta1.HookTypeWorkflow}},
					},
				},
			},
			want: true,
		},
		{
			name: "scale prehook Workflow type",
			curator: ccv1beta1.ClusterCurator{
				Spec: ccv1beta1.ClusterCuratorSpec{
					Scale: ccv1beta1.Hooks{
						Prehook: []ccv1beta1.Hook{{Type: ccv1beta1.HookTypeWorkflow}},
					},
				},
			},
			want: true,
		},
		{
			name: "scale posthook Workflow type",
			curator: ccv1beta1.ClusterCurator{
				Spec: ccv1beta1.ClusterCuratorSpec{
					Scale: ccv1beta1.Hooks{
						Posthook: []ccv1beta1.Hook{{Type: ccv1beta1.HookTypeWorkflow}},
					},
				},
			},
			want: true,
		},
		{
			name: "upgrade prehook Workflow type",
			curator: ccv1beta1.ClusterCurator{
				Spec: ccv1beta1.ClusterCuratorSpec{
					Upgrade: ccv1beta1.UpgradeHooks{
						Prehook: []ccv1beta1.Hook{{Type: ccv1beta1.HookTypeWorkflow}},
					},
				},
			},
			want: true,
		},
		{
			name: "upgrade posthook Workflow type",
			curator: ccv1beta1.ClusterCurator{
				Spec: ccv1beta1.ClusterCuratorSpec{
					Upgrade: ccv1beta1.UpgradeHooks{
						Posthook: []ccv1beta1.Hook{{Type: ccv1beta1.HookTypeWorkflow}},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsWorkflowType(tt.curator); got != tt.want {
				t.Errorf("containsWorkflowType() = %v, want %v", got, tt.want)
			}
		})
	}
}
