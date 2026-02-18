package basic

import (
	"fmt"
	"testing"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/giantswarm/apptest-framework/v2/pkg/state"
	"github.com/giantswarm/apptest-framework/v2/pkg/suite"
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
	isUpgrade = false

	efsProvisioner = "efs.csi.aws.com"
	testNamespace  = "default"
	scName         = "efs-dynamic-e2e"
)

// Shared state between hooks and tests.
var efs *efsInfra

func TestBasic(t *testing.T) {
	suite.New().
		WithInCluster(true).
		WithInstallNamespace("").
		WithIsUpgrade(isUpgrade).
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
		Tests(func() {
			It("should have the HelmRelease ready on the management cluster", func() {
				mcClient := state.GetFramework().MC()
				clusterName := state.GetCluster().Name
				orgName := state.GetCluster().Organization.Name

				Eventually(func() (bool, error) {
					return helmReleaseIsReady(mcClient, clusterName, orgName)
				}).
					WithTimeout(15 * time.Minute).
					WithPolling(10 * time.Second).
					Should(BeTrue())
			})

			It("should have the efs-csi-controller deployment running", func() {
				wcClient, err := state.GetFramework().WC(state.GetCluster().Name)
				Expect(err).Should(Succeed())

				Eventually(func() error {
					var dp appsv1.Deployment
					return wcClient.Get(state.GetContext(), types.NamespacedName{
						Namespace: "kube-system",
						Name:      "efs-csi-controller",
					}, &dp)
				}).
					WithTimeout(10 * time.Minute).
					WithPolling(5 * time.Second).
					ShouldNot(HaveOccurred())
			})

			It("should have the efs-csi-node daemonset running", func() {
				wcClient, err := state.GetFramework().WC(state.GetCluster().Name)
				Expect(err).Should(Succeed())

				Eventually(func() error {
					var ds appsv1.DaemonSet
					return wcClient.Get(state.GetContext(), types.NamespacedName{
						Namespace: "kube-system",
						Name:      "efs-csi-node",
					}, &ds)
				}).
					WithTimeout(10 * time.Minute).
					WithPolling(5 * time.Second).
					ShouldNot(HaveOccurred())
			})

			It("should dynamically provision an EFS volume and allow shared read-write access", func() {
				Expect(efs).NotTo(BeNil(), "EFS infrastructure was not created")
				Expect(efs.fileSystemID).NotTo(BeEmpty(), "EFS filesystem ID is not available")

				wcClient, err := state.GetFramework().WC(state.GetCluster().Name)
				Expect(err).Should(Succeed())
				ctx := state.GetContext()

				pvcName := "efs-claim-e2e"
				writerPodName := "efs-writer-e2e"
				readerPodName := "efs-reader-e2e"
				testData := "efs-dynamic-provisioning-works"

				By("Creating a StorageClass for EFS dynamic provisioning")
				bindingMode := storagev1.VolumeBindingImmediate
				reclaimPolicy := corev1.PersistentVolumeReclaimDelete
				sc := &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: scName,
					},
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

				By("Creating a PVC that uses the EFS StorageClass")
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvcName,
						Namespace: testNamespace,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
						StorageClassName: &scName,
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("5Gi"),
							},
						},
					},
				}
				Expect(wcClient.Create(ctx, pvc)).To(Succeed())

				By("Creating a writer Pod that mounts the EFS volume and writes data")
				writerPod := newTestPod(writerPodName, testNamespace, pvcName,
					[]string{"sh", "-c", fmt.Sprintf("echo '%s' > /data/testfile && echo 'write-ok'", testData)},
				)
				Expect(wcClient.Create(ctx, writerPod)).To(Succeed())

				By("Waiting for the writer Pod to succeed")
				Eventually(func() (corev1.PodPhase, error) {
					var pod corev1.Pod
					err := wcClient.Get(ctx, types.NamespacedName{
						Name:      writerPodName,
						Namespace: testNamespace,
					}, &pod)
					if err != nil {
						return "", err
					}
					return pod.Status.Phase, nil
				}).
					WithTimeout(10 * time.Minute).
					WithPolling(5 * time.Second).
					Should(Equal(corev1.PodSucceeded))

				By("Verifying the PVC is bound")
				var claim corev1.PersistentVolumeClaim
				Expect(wcClient.Get(ctx, types.NamespacedName{
					Name:      pvcName,
					Namespace: testNamespace,
				}, &claim)).To(Succeed())
				Expect(claim.Status.Phase).To(Equal(corev1.ClaimBound))

				By("Creating a reader Pod that reads data from the same EFS volume")
				readerPod := newTestPod(readerPodName, testNamespace, pvcName,
					[]string{"sh", "-c", fmt.Sprintf("cat /data/testfile | grep '%s'", testData)},
				)
				Expect(wcClient.Create(ctx, readerPod)).To(Succeed())

				By("Waiting for the reader Pod to succeed, confirming shared access works")
				Eventually(func() (corev1.PodPhase, error) {
					var pod corev1.Pod
					err := wcClient.Get(ctx, types.NamespacedName{
						Name:      readerPodName,
						Namespace: testNamespace,
					}, &pod)
					if err != nil {
						return "", err
					}
					return pod.Status.Phase, nil
				}).
					WithTimeout(5 * time.Minute).
					WithPolling(5 * time.Second).
					Should(Equal(corev1.PodSucceeded))
			})
		}).
		AfterSuite(func() {
			It("should clean up test resources", func() {
				ctx := state.GetContext()

				// Clean up WC resources while the CSI driver is still running,
				// so that PVC deletion triggers access point removal.
				wcClient, err := state.GetFramework().WC(state.GetCluster().Name)
				if err == nil {
					for _, name := range []string{"efs-reader-e2e", "efs-writer-e2e"} {
						pod := &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
						}
						_ = client.IgnoreNotFound(wcClient.Delete(ctx, pod))
					}

					pvc := &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{Name: "efs-claim-e2e", Namespace: testNamespace},
					}
					_ = client.IgnoreNotFound(wcClient.Delete(ctx, pvc))

					// Wait for PVC to be fully deleted so the CSI driver can
					// clean up the access point before we tear down the filesystem.
					Eventually(func() bool {
						err := wcClient.Get(ctx, types.NamespacedName{
							Name:      "efs-claim-e2e",
							Namespace: testNamespace,
						}, &corev1.PersistentVolumeClaim{})
						return apierrors.IsNotFound(err)
					}).WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).Should(BeTrue())

					sc := &storagev1.StorageClass{
						ObjectMeta: metav1.ObjectMeta{Name: scName},
					}
					_ = client.IgnoreNotFound(wcClient.Delete(ctx, sc))
				}

				// Clean up Crossplane EFS resources on the MC.
				if efs != nil {
					mcClient := state.GetFramework().MC()
					efs.Cleanup(ctx, *mcClient)
				}
			})
		}).
		Run(t, "EFS Dynamic Provisioning")
}

func helmReleaseIsReady(mcClient *client.Client, clusterName, orgName string) (bool, error) {
	hr := &helmv2.HelmRelease{}
	err := (*mcClient).Get(state.GetContext(), types.NamespacedName{
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

func newTestPod(name, namespace, pvcName string, command []string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "test",
					Image:   "busybox:1.36",
					Command: command,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "efs-volume",
							MountPath: "/data",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "efs-volume",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}
}
