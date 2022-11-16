// Copyright Contributors to the Open Cluster Management project.

package controllers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ccv1beta1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	ccNoWorkflowName = "cc-no-workflow"
	ccWorkflowName   = "cc-workflow"
	ccNamespace      = "default"
)

var _ = Describe("Cluster Curator controller", func() {
	ccNoWorkflowKey := types.NamespacedName{Name: ccNoWorkflowName, Namespace: ccNamespace}
	ccWorkflowKey := types.NamespacedName{Name: ccWorkflowName, Namespace: ccNamespace}

	Context("When ClusterCurator is created", func() {
		It("Should check AnsibleJob CRD supports Workflow invokcation", func() {
			By("Creating the ClusterCurator without Workflow type")
			ctx := context.Background()
			ccNoWorkflow := ccv1beta1.ClusterCurator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ccNoWorkflowName,
					Namespace: ccNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, &ccNoWorkflow)).Should(Succeed())
			ccNoWorkflow = ccv1beta1.ClusterCurator{}
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, ccNoWorkflowKey, &ccNoWorkflow); err != nil {
					return false
				}
				return !meta.IsStatusConditionPresentAndEqual(ccNoWorkflow.Status.Conditions,
					statusTypeInitiated, metav1.ConditionFalse)
			}).Should(BeTrue())

			By("Creating the ClusterCurator with Workflow type")
			ccWorkflow := ccv1beta1.ClusterCurator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ccWorkflowName,
					Namespace: ccNamespace,
				},
				Spec: ccv1beta1.ClusterCuratorSpec{
					Install: ccv1beta1.Hooks{Prehook: []ccv1beta1.Hook{{
						Name: "prehook-name",
						Type: ccv1beta1.HookTypeWorkflow}}},
				},
			}
			Expect(k8sClient.Create(ctx, &ccWorkflow)).Should(Succeed())
			ccWorkflow = ccv1beta1.ClusterCurator{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, ccWorkflowKey, &ccWorkflow); err != nil {
					return false
				}
				return meta.IsStatusConditionPresentAndEqual(ccWorkflow.Status.Conditions,
					statusTypeInitiated, metav1.ConditionTrue)
			}).Should(BeTrue())
		})
	})
})
