package upgrade

import (
	"context"
	"fmt"
	"strings"
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
		providerConfig: clusterName,
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
		// resourceID is the actual AWS subnet ID (subnet-xxx),
		// while id is the CAPI name (clustername-subnet-private-az).
		id, _, _ := unstructured.NestedString(sub, "resourceID")
		if id == "" {
			id, _, _ = unstructured.NestedString(sub, "id")
		}
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

	GinkgoLogr.Info("discovered network",
		"region", e.region,
		"vpcID", e.vpcID,
		"vpcCIDR", e.vpcCIDR,
		"privateSubnets", len(e.privateSubnets),
		"providerConfig", e.providerConfig,
	)

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
	GinkgoLogr.Info("using providerConfig", "name", e.providerConfig)
}

// Create provisions EFS infrastructure via Crossplane on the MC.
// It creates a SecurityGroup, FileSystem, ingress rule, and MountTargets,
// then waits for all resources to become ready.
func (e *efsInfra) Create(ctx context.Context, c client.Client) {
	prefix := e.clusterName + "-efs-e2e"

	By("Creating Crossplane SecurityGroup")
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
	GinkgoLogr.Info("created SecurityGroup", "name", prefix+"-sg")

	By("Creating Crossplane FileSystem")
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
	GinkgoLogr.Info("created FileSystem", "name", prefix+"-fs")

	By("Waiting for SecurityGroup AWS ID")
	Eventually(func() string {
		e.securityGroupID = getAtProviderID(ctx, c, ec2SecurityGroupGVK, prefix+"-sg")
		if e.securityGroupID != "" {
			GinkgoLogr.Info("SecurityGroup has AWS ID", "name", prefix+"-sg", "id", e.securityGroupID)
		} else {
			logResourceStatus(ctx, c, ec2SecurityGroupGVK, prefix+"-sg")
		}
		return e.securityGroupID
	}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).ShouldNot(BeEmpty())

	By("Waiting for SecurityGroup to be ready")
	Eventually(func() bool {
		return isResourceReady(ctx, c, ec2SecurityGroupGVK, prefix+"-sg")
	}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(BeTrue())

	By("Waiting for FileSystem AWS ID")
	Eventually(func() string {
		e.fileSystemID = getAtProviderID(ctx, c, efsFileSystemGVK, prefix+"-fs")
		if e.fileSystemID != "" {
			GinkgoLogr.Info("FileSystem has AWS ID", "name", prefix+"-fs", "id", e.fileSystemID)
		} else {
			logResourceStatus(ctx, c, efsFileSystemGVK, prefix+"-fs")
		}
		return e.fileSystemID
	}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).ShouldNot(BeEmpty())

	By("Waiting for FileSystem to be ready")
	Eventually(func() bool {
		return isResourceReady(ctx, c, efsFileSystemGVK, prefix+"-fs")
	}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(BeTrue())

	By("Creating SecurityGroup ingress rule for NFS (port 2049)")
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
	GinkgoLogr.Info("created SecurityGroupRule", "name", prefix+"-sgr-nfs", "sgID", e.securityGroupID, "cidr", e.vpcCIDR)

	By("Waiting for SecurityGroupRule to be ready")
	Eventually(func() bool {
		return isResourceReady(ctx, c, ec2SecurityGroupRuleGVK, prefix+"-sgr-nfs")
	}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(BeTrue())

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
		GinkgoLogr.Info("created MountTarget", "name", mtName, "subnet", subnet.id, "az", subnet.az, "fsID", e.fileSystemID)
	}

	By("Waiting for all MountTargets to be ready")
	for _, subnet := range e.privateSubnets {
		mtName := prefix + "-mt-" + subnet.az
		By(fmt.Sprintf("Waiting for MountTarget %s (%s) to be ready", mtName, subnet.az))
		Eventually(func() bool {
			return isResourceReady(ctx, c, efsMountTargetGVK, mtName)
		}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(BeTrue())
	}

	GinkgoLogr.Info("all EFS infrastructure is ready",
		"fileSystemID", e.fileSystemID,
		"securityGroupID", e.securityGroupID,
		"mountTargets", len(e.privateSubnets),
	)
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
		} else {
			GinkgoLogr.Info("deleting resource", "kind", ref.gvk.Kind, "name", ref.name)
		}
	}

	By("Waiting for Crossplane resources to be removed")
	for _, ref := range e.created {
		By(fmt.Sprintf("Waiting for %s/%s to be deleted", ref.gvk.Kind, ref.name))
		Eventually(func() bool {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(ref.gvk)
			err := c.Get(ctx, types.NamespacedName{Name: ref.name}, obj)
			if apierrors.IsNotFound(err) {
				GinkgoLogr.Info("resource deleted", "kind", ref.gvk.Kind, "name", ref.name)
				return true
			}
			logResourceStatus(ctx, c, ref.gvk, ref.name)
			return false
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
		GinkgoLogr.Info("resource not found", "kind", gvk.Kind, "name", name, "error", err.Error())
		return ""
	}
	id, _, _ := unstructured.NestedString(obj.Object, "status", "atProvider", "id")
	return id
}

// logResourceStatus logs all conditions for a Crossplane resource.
func logResourceStatus(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, name string) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	if err := c.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
		GinkgoLogr.Info("cannot fetch resource status", "kind", gvk.Kind, "name", name, "error", err.Error())
		return
	}
	conditions, ok, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !ok || len(conditions) == 0 {
		GinkgoLogr.Info("resource has no conditions yet", "kind", gvk.Kind, "name", name)
		return
	}
	var parts []string
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		t, _, _ := unstructured.NestedString(cond, "type")
		s, _, _ := unstructured.NestedString(cond, "status")
		reason, _, _ := unstructured.NestedString(cond, "reason")
		msg, _, _ := unstructured.NestedString(cond, "message")
		parts = append(parts, fmt.Sprintf("%s=%s (%s: %s)", t, s, reason, msg))
	}
	GinkgoLogr.Info("resource status", "kind", gvk.Kind, "name", name, "conditions", strings.Join(parts, " | "))
}

func isResourceReady(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, name string) bool {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	if err := c.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
		GinkgoLogr.Info("resource not found", "kind", gvk.Kind, "name", name, "error", err.Error())
		return false
	}
	conditions, ok, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !ok {
		GinkgoLogr.Info("resource has no conditions yet", "kind", gvk.Kind, "name", name)
		return false
	}
	var parts []string
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		t, _, _ := unstructured.NestedString(cond, "type")
		s, _, _ := unstructured.NestedString(cond, "status")
		reason, _, _ := unstructured.NestedString(cond, "reason")
		msg, _, _ := unstructured.NestedString(cond, "message")
		parts = append(parts, fmt.Sprintf("%s=%s (%s: %s)", t, s, reason, msg))
		if t == "Ready" && s == "True" {
			GinkgoLogr.Info("resource is ready", "kind", gvk.Kind, "name", name)
			return true
		}
	}
	GinkgoLogr.Info("resource not ready", "kind", gvk.Kind, "name", name, "conditions", strings.Join(parts, " | "))
	return false
}
