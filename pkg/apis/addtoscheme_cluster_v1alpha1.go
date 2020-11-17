// Copyright (c) 2020 Red Hat, Inc.

package apis

import v1alpha1 "github.com/open-cluster-management/cluster-curator-controller/pkg/apis/cluster/v1alpha1"

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes, v1alpha1.SchemeBuilder.AddToScheme)
}
