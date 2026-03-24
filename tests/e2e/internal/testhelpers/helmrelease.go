package testhelpers

import (
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/giantswarm/apptest-framework/v2/pkg/state"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HelmReleaseIsReady checks if the EFS CSI driver HelmRelease is ready on the MC.
func HelmReleaseIsReady(mcClient client.Client, clusterName, orgName string) (bool, error) {
	hr := &helmv2.HelmRelease{}
	err := mcClient.Get(state.GetContext(), types.NamespacedName{
		Name:      clusterName + "-aws-efs-csi-driver",
		Namespace: "org-" + orgName,
	}, hr)
	if err != nil {
		return false, err
	}
	for _, cond := range hr.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == metav1.ConditionTrue {
			return true, nil
		}
	}
	return false, nil
}
