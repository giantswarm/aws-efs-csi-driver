package basic

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	awsClusterGVK = schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Version: "v1beta2",
		Kind:    "AWSCluster",
	}

	efsFileSystemGVK = schema.GroupVersionKind{
		Group:   "efs.aws.upbound.io",
		Version: "v1beta1",
		Kind:    "FileSystem",
	}

	efsMountTargetGVK = schema.GroupVersionKind{
		Group:   "efs.aws.upbound.io",
		Version: "v1beta1",
		Kind:    "MountTarget",
	}

	ec2SecurityGroupGVK = schema.GroupVersionKind{
		Group:   "ec2.aws.upbound.io",
		Version: "v1beta1",
		Kind:    "SecurityGroup",
	}

	ec2SecurityGroupRuleGVK = schema.GroupVersionKind{
		Group:   "ec2.aws.upbound.io",
		Version: "v1beta1",
		Kind:    "SecurityGroupRule",
	}
)

type efsInfra struct {
	clusterName    string
	orgNamespace   string
	region         string
	providerConfig string
	vpcID          string
	vpcCIDR        string
	privateSubnets []subnetInfo

	fileSystemID    string
	securityGroupID string

	// Track created resources for cleanup (in creation order).
	created []resourceRef
}

type subnetInfo struct {
	id string
	az string
}

type resourceRef struct {
	gvk  schema.GroupVersionKind
	name string
}

func newEFSInfra(clusterName, orgNamespace string) *efsInfra {
	return &efsInfra{
		clusterName:    clusterName,
		orgNamespace:   orgNamespace,
		providerConfig: "default",
	}
}

// DiscoverNetwork reads the AWSCluster resource to extract VPC, subnets, and region.
func (e *efsInfra) DiscoverNetwork(ctx context.Context, c client.Client) error {
	awsCluster := &unstructured.Unstructured{}
	awsCluster.SetGroupVersionKind(awsClusterGVK)

	if err := c.Get(ctx, types.NamespacedName{
		Name:      e.clusterName,
		Namespace: e.orgNamespace,
	}, awsCluster); err != nil {
		return fmt.Errorf("getting AWSCluster %s/%s: %w", e.orgNamespace, e.clusterName, err)
	}

	// Region
	region, ok, _ := unstructured.NestedString(awsCluster.Object, "spec", "region")
	if !ok || region == "" {
		return fmt.Errorf("AWSCluster missing spec.region")
	}
	e.region = region

	// VPC ID — try status first, then spec
	vpcID, _, _ := unstructured.NestedString(awsCluster.Object, "status", "networkStatus", "vpc", "id")
	if vpcID == "" {
		vpcID, _, _ = unstructured.NestedString(awsCluster.Object, "spec", "network", "vpc", "id")
	}
	if vpcID == "" {
		return fmt.Errorf("could not find VPC ID in AWSCluster status or spec")
	}
	e.vpcID = vpcID

	// VPC CIDR — for security group rule
	cidr, _, _ := unstructured.NestedString(awsCluster.Object, "status", "networkStatus", "vpc", "cidrBlock")
	if cidr == "" {
		cidr, _, _ = unstructured.NestedString(awsCluster.Object, "spec", "network", "vpc", "cidrBlock")
	}
	if cidr == "" {
		cidr = "0.0.0.0/0"
	}
	e.vpcCIDR = cidr

	// Subnets — try status first, then spec
	subnets, ok, _ := unstructured.NestedSlice(awsCluster.Object, "status", "networkStatus", "subnets")
	if !ok || len(subnets) == 0 {
		subnets, ok, _ = unstructured.NestedSlice(awsCluster.Object, "spec", "network", "subnets")
	}
	if !ok || len(subnets) == 0 {
		return fmt.Errorf("no subnets found in AWSCluster")
	}

	// Keep one private subnet per AZ for mount targets.
	seenAZs := map[string]bool{}
	for _, s := range subnets {
		sub, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		isPublic, _, _ := unstructured.NestedBool(sub, "isPublic")
		if isPublic {
			continue
		}
		id, _, _ := unstructured.NestedString(sub, "id")
		az, _, _ := unstructured.NestedString(sub, "availabilityZone")
		if id == "" || az == "" || seenAZs[az] {
			continue
		}
		seenAZs[az] = true
		e.privateSubnets = append(e.privateSubnets, subnetInfo{id: id, az: az})
	}
	if len(e.privateSubnets) == 0 {
		return fmt.Errorf("no private subnets found in AWSCluster")
	}

	return nil
}

// DiscoverProviderConfig reads the crossplane-config ConfigMap for the cluster.
func (e *efsInfra) DiscoverProviderConfig(ctx context.Context, c client.Client) {
	var cm corev1.ConfigMap
	key := types.NamespacedName{
		Name:      e.clusterName + "-crossplane-config",
		Namespace: e.orgNamespace,
	}
	if err := c.Get(ctx, key, &cm); err == nil {
		if name, ok := cm.Data["providerConfigName"]; ok && name != "" {
			e.providerConfig = name
		}
	}
}

// Create provisions EFS infrastructure via Crossplane on the MC.
// It creates a SecurityGroup, FileSystem, ingress rule, and MountTargets,
// then waits for all resources to become ready.
func (e *efsInfra) Create(ctx context.Context, c client.Client) {
	prefix := e.clusterName + "-efs-e2e"

	By("Creating Crossplane SecurityGroup and FileSystem")
	sg := newCrossplaneResource(ec2SecurityGroupGVK, prefix+"-sg", map[string]interface{}{
		"forProvider": map[string]interface{}{
			"region":      e.region,
			"vpcId":       e.vpcID,
			"description": "EFS E2E test - NFS access",
			"tags": map[string]interface{}{
				"Name": prefix + "-sg",
			},
		},
		"providerConfigRef": map[string]interface{}{
			"name": e.providerConfig,
		},
	})
	Expect(c.Create(ctx, sg)).To(Succeed())
	e.track(ec2SecurityGroupGVK, prefix+"-sg")

	fs := newCrossplaneResource(efsFileSystemGVK, prefix+"-fs", map[string]interface{}{
		"forProvider": map[string]interface{}{
			"region":          e.region,
			"performanceMode": "generalPurpose",
			"tags": map[string]interface{}{
				"Name": prefix + "-fs",
			},
		},
		"providerConfigRef": map[string]interface{}{
			"name": e.providerConfig,
		},
	})
	Expect(c.Create(ctx, fs)).To(Succeed())
	e.track(efsFileSystemGVK, prefix+"-fs")

	By("Waiting for SecurityGroup and FileSystem AWS IDs")
	Eventually(func() string {
		e.securityGroupID = getAtProviderID(ctx, c, ec2SecurityGroupGVK, prefix+"-sg")
		return e.securityGroupID
	}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).ShouldNot(BeEmpty())

	Eventually(func() string {
		e.fileSystemID = getAtProviderID(ctx, c, efsFileSystemGVK, prefix+"-fs")
		return e.fileSystemID
	}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).ShouldNot(BeEmpty())

	By("Creating SecurityGroup ingress rule for NFS")
	sgr := newCrossplaneResource(ec2SecurityGroupRuleGVK, prefix+"-sgr-nfs", map[string]interface{}{
		"forProvider": map[string]interface{}{
			"region":          e.region,
			"securityGroupId": e.securityGroupID,
			"type":            "ingress",
			"fromPort":        float64(2049),
			"toPort":          float64(2049),
			"protocol":        "tcp",
			"cidrBlocks":      []interface{}{e.vpcCIDR},
		},
		"providerConfigRef": map[string]interface{}{
			"name": e.providerConfig,
		},
	})
	Expect(c.Create(ctx, sgr)).To(Succeed())
	e.track(ec2SecurityGroupRuleGVK, prefix+"-sgr-nfs")

	By(fmt.Sprintf("Creating MountTargets in %d private subnets", len(e.privateSubnets)))
	for _, subnet := range e.privateSubnets {
		mtName := prefix + "-mt-" + subnet.az
		mt := newCrossplaneResource(efsMountTargetGVK, mtName, map[string]interface{}{
			"forProvider": map[string]interface{}{
				"region":         e.region,
				"fileSystemId":   e.fileSystemID,
				"subnetId":       subnet.id,
				"securityGroups": []interface{}{e.securityGroupID},
			},
			"providerConfigRef": map[string]interface{}{
				"name": e.providerConfig,
			},
		})
		Expect(c.Create(ctx, mt)).To(Succeed())
		e.track(efsMountTargetGVK, mtName)
	}

	By("Waiting for MountTargets to be ready")
	for _, subnet := range e.privateSubnets {
		mtName := prefix + "-mt-" + subnet.az
		Eventually(func() bool {
			return isResourceReady(ctx, c, efsMountTargetGVK, mtName)
		}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(BeTrue())
	}
}

// Cleanup deletes all Crossplane resources in reverse creation order and
// waits for them to be fully removed.
func (e *efsInfra) Cleanup(ctx context.Context, c client.Client) {
	By("Deleting Crossplane EFS resources")
	for i := len(e.created) - 1; i >= 0; i-- {
		ref := e.created[i]
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(ref.gvk)
		obj.SetName(ref.name)
		err := c.Delete(ctx, obj)
		if err != nil && !apierrors.IsNotFound(err) {
			GinkgoLogr.Error(err, "failed to delete", "kind", ref.gvk.Kind, "name", ref.name)
		}
	}

	By("Waiting for Crossplane resources to be removed")
	for _, ref := range e.created {
		Eventually(func() bool {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(ref.gvk)
			err := c.Get(ctx, types.NamespacedName{Name: ref.name}, obj)
			return apierrors.IsNotFound(err)
		}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(BeTrue())
	}
}

func (e *efsInfra) track(gvk schema.GroupVersionKind, name string) {
	e.created = append(e.created, resourceRef{gvk: gvk, name: name})
}

func newCrossplaneResource(gvk schema.GroupVersionKind, name string, spec map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName(name)
	obj.Object["spec"] = spec
	return obj
}

func getAtProviderID(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, name string) string {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	if err := c.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
		return ""
	}
	id, _, _ := unstructured.NestedString(obj.Object, "status", "atProvider", "id")
	return id
}

func isResourceReady(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, name string) bool {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	if err := c.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
		return false
	}
	conditions, ok, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !ok {
		return false
	}
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		t, _, _ := unstructured.NestedString(cond, "type")
		s, _, _ := unstructured.NestedString(cond, "status")
		if t == "Ready" && s == "True" {
			return true
		}
	}
	return false
}
