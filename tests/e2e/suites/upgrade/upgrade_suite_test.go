package upgrade

import (
	"testing"
	"time"

	"e2e/internal/testhelpers"

	"github.com/giantswarm/apptest-framework/v2/pkg/state"
	"github.com/giantswarm/apptest-framework/v2/pkg/suite"
	"github.com/giantswarm/clustertest/v2/pkg/failurehandler"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	efsProvisioner = "efs.csi.aws.com"
	testNamespace  = "default"
	scName         = "efs-upgrade-e2e"
)

var efs *efsInfra

func TestUpgrade(t *testing.T) {
	suite.New().
		WithInCluster(true).
		WithInstallNamespace("").
		WithIsUpgrade(true).
		WithValuesFile("./values.yaml").
		AfterClusterReady(func() {
			It("should create EFS infrastructure via Crossplane", func() {
				mcClient := state.GetFramework().MC()
				ctx := state.GetContext()
				cluster := state.GetCluster()

				efs = newEFSInfra(cluster.Name, "org-"+cluster.Organization.Name)
				efs.DiscoverProviderConfig(ctx, *mcClient)
				Expect(efs.DiscoverNetwork(ctx, *mcClient)).To(Succeed())
				efs.Create(ctx, *mcClient)
			})
		}).
		BeforeUpgrade(func() {
			It("should have the HelmRelease ready before upgrade", func() {
				mcClient := state.GetFramework().MC()
				clusterName := state.GetCluster().Name
				orgName := state.GetCluster().Organization.Name

				Eventually(func() (bool, error) {
					ready, err := testhelpers.HelmReleaseIsReady(*mcClient, clusterName, orgName)
					if err != nil {
						GinkgoLogr.Info("HelmRelease check failed", "error", err.Error())
					} else if !ready {
						GinkgoLogr.Info("HelmRelease not ready yet", "name", clusterName+"-aws-efs-csi-driver")
					} else {
						GinkgoLogr.Info("HelmRelease is ready", "name", clusterName+"-aws-efs-csi-driver")
					}
					return ready, err
				}).
					WithTimeout(15 * time.Minute).
					WithPolling(10 * time.Second).
					Should(BeTrue())
			})

			It("should write data to EFS before upgrade", func() {
				Expect(efs).NotTo(BeNil(), "EFS infrastructure was not created")
				Expect(efs.fileSystemID).NotTo(BeEmpty(), "EFS filesystem ID is not available")

				wcClient, err := state.GetFramework().WC(state.GetCluster().Name)
				Expect(err).Should(Succeed())
				ctx := state.GetContext()

				By("Creating a StorageClass for EFS dynamic provisioning")
				bindingMode := storagev1.VolumeBindingImmediate
				reclaimPolicy := corev1.PersistentVolumeReclaimDelete
				sc := &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{Name: scName},
					Provisioner:       efsProvisioner,
					VolumeBindingMode: &bindingMode,
					ReclaimPolicy:     &reclaimPolicy,
					Parameters: map[string]string{
						"provisioningMode": "efs-ap",
						"fileSystemId":     efs.fileSystemID,
						"directoryPerms":   "700",
					},
				}
				Expect(wcClient.Create(ctx, sc)).To(Succeed())

				By("Creating a PVC")
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "efs-claim-upgrade-e2e",
						Namespace: testNamespace,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
						StorageClassName: ptr(scName),
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("5Gi"),
							},
						},
					},
				}
				Expect(wcClient.Create(ctx, pvc)).To(Succeed())

				By("Writing data before upgrade")
				writerPod := testhelpers.NewTestPod("efs-pre-upgrade-writer", testNamespace, "efs-claim-upgrade-e2e",
					[]string{"sh", "-c", "echo 'pre-upgrade-data' > /data/upgrade-test && echo 'write-ok'"},
				)
				Expect(wcClient.Create(ctx, writerPod)).To(Succeed())

				Eventually(func() (corev1.PodPhase, error) {
					var pod corev1.Pod
					err := wcClient.Get(ctx, types.NamespacedName{
						Name:      "efs-pre-upgrade-writer",
						Namespace: testNamespace,
					}, &pod)
					if err != nil {
						return "", err
					}
					GinkgoLogr.Info("pre-upgrade writer pod", "phase", pod.Status.Phase)
					return pod.Status.Phase, nil
				}).
					WithTimeout(10 * time.Minute).
					WithPolling(5 * time.Second).
					Should(Equal(corev1.PodSucceeded))
			})
		}).
		Tests(func() {
			It("should have the HelmRelease ready after upgrade", func() {
				mcClient := state.GetFramework().MC()
				clusterName := state.GetCluster().Name
				orgName := state.GetCluster().Organization.Name

				Eventually(func() (bool, error) {
					ready, err := testhelpers.HelmReleaseIsReady(*mcClient, clusterName, orgName)
					if err != nil {
						GinkgoLogr.Info("HelmRelease check failed", "error", err.Error())
					} else if !ready {
						GinkgoLogr.Info("HelmRelease not ready yet after upgrade")
					} else {
						GinkgoLogr.Info("HelmRelease is ready after upgrade")
					}
					return ready, err
				}).
					WithTimeout(15 * time.Minute).
					WithPolling(10 * time.Second).
					Should(BeTrue(), failurehandler.LLMPrompt(state.GetFramework(), state.GetCluster(), "Investigate HelmRelease not ready after upgrade"))
			})

			It("should have the efs-csi-controller running after upgrade", func() {
				wcClient, err := state.GetFramework().WC(state.GetCluster().Name)
				Expect(err).Should(Succeed())

				Eventually(func() error {
					var dp appsv1.Deployment
					err := wcClient.Get(state.GetContext(), types.NamespacedName{
						Namespace: "kube-system",
						Name:      "efs-csi-controller",
					}, &dp)
					if err != nil {
						GinkgoLogr.Info("efs-csi-controller not found", "error", err.Error())
					} else {
						GinkgoLogr.Info("efs-csi-controller", "ready", dp.Status.ReadyReplicas, "replicas", dp.Status.Replicas)
					}
					return err
				}).
					WithTimeout(10 * time.Minute).
					WithPolling(5 * time.Second).
					ShouldNot(HaveOccurred())
			})

			It("should still have access to data written before upgrade", func() {
				wcClient, err := state.GetFramework().WC(state.GetCluster().Name)
				Expect(err).Should(Succeed())
				ctx := state.GetContext()

				By("Reading data written before upgrade")
				readerPod := testhelpers.NewTestPod("efs-post-upgrade-reader", testNamespace, "efs-claim-upgrade-e2e",
					[]string{"sh", "-c", "cat /data/upgrade-test | grep 'pre-upgrade-data'"},
				)
				Expect(wcClient.Create(ctx, readerPod)).To(Succeed())

				Eventually(func() (corev1.PodPhase, error) {
					var pod corev1.Pod
					err := wcClient.Get(ctx, types.NamespacedName{
						Name:      "efs-post-upgrade-reader",
						Namespace: testNamespace,
					}, &pod)
					if err != nil {
						return "", err
					}
					GinkgoLogr.Info("post-upgrade reader pod", "phase", pod.Status.Phase)
					return pod.Status.Phase, nil
				}).
					WithTimeout(5 * time.Minute).
					WithPolling(5 * time.Second).
					Should(Equal(corev1.PodSucceeded), failurehandler.LLMPrompt(state.GetFramework(), state.GetCluster(), "Investigate post-upgrade reader pod - data written before upgrade should survive"))
			})

			It("should write new data after upgrade", func() {
				wcClient, err := state.GetFramework().WC(state.GetCluster().Name)
				Expect(err).Should(Succeed())
				ctx := state.GetContext()

				By("Writing new data after upgrade")
				writerPod := testhelpers.NewTestPod("efs-post-upgrade-writer", testNamespace, "efs-claim-upgrade-e2e",
					[]string{"sh", "-c", "echo 'post-upgrade-data' > /data/post-upgrade-test && echo 'write-ok'"},
				)
				Expect(wcClient.Create(ctx, writerPod)).To(Succeed())

				Eventually(func() (corev1.PodPhase, error) {
					var pod corev1.Pod
					err := wcClient.Get(ctx, types.NamespacedName{
						Name:      "efs-post-upgrade-writer",
						Namespace: testNamespace,
					}, &pod)
					if err != nil {
						return "", err
					}
					GinkgoLogr.Info("post-upgrade writer pod", "phase", pod.Status.Phase)
					return pod.Status.Phase, nil
				}).
					WithTimeout(10 * time.Minute).
					WithPolling(5 * time.Second).
					Should(Equal(corev1.PodSucceeded), failurehandler.LLMPrompt(state.GetFramework(), state.GetCluster(), "Investigate post-upgrade writer pod - new writes should work after upgrade"))
			})
		}).
		AfterSuite(func() {
			ctx := state.GetContext()

			wcClient, err := state.GetFramework().WC(state.GetCluster().Name)
			if err == nil {
				for _, name := range []string{
					"efs-pre-upgrade-writer",
					"efs-post-upgrade-reader",
					"efs-post-upgrade-writer",
				} {
					pod := &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
					}
					_ = client.IgnoreNotFound(wcClient.Delete(ctx, pod))
				}

				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{Name: "efs-claim-upgrade-e2e", Namespace: testNamespace},
				}
				_ = client.IgnoreNotFound(wcClient.Delete(ctx, pvc))

				Eventually(func() bool {
					err := wcClient.Get(ctx, types.NamespacedName{
						Name:      "efs-claim-upgrade-e2e",
						Namespace: testNamespace,
					}, &corev1.PersistentVolumeClaim{})
					return apierrors.IsNotFound(err)
				}).WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).Should(BeTrue())

				sc := &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{Name: scName},
				}
				_ = client.IgnoreNotFound(wcClient.Delete(ctx, sc))
			}

			if efs != nil {
				mcClient := state.GetFramework().MC()
				efs.Cleanup(ctx, *mcClient)
			}
		}).
		Run(t, "EFS Upgrade")
}

func ptr[T any](v T) *T { return &v }
