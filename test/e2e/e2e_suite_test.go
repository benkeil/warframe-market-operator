/*
Copyright 2026.

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

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tck3s "github.com/testcontainers/testcontainers-go/modules/k3s"

	"github.com/benkeil/warframe-market-operator/test/utils"
)

var (
	projectImage   = "example.com/warframe-market-operator:v0.0.1"
	k3sContainer   *tck3s.K3sContainer
	kubeconfigFile string
)

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment via a k3s Testcontainer — no external cluster or Kind required.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting warframe-market-operator integration test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	ctx := context.Background()

	By("building the manager(Operator) image")
	cmd := exec.Command("just", fmt.Sprintf("img=%s", projectImage), "docker-build")
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager(Operator) image")

	By("starting k3s via Testcontainers")
	k3sContainer, err = tck3s.Run(ctx, "rancher/k3s:v1.33.13-k3s1")
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to start k3s container")

	By("loading the manager image into k3s")
	err = k3sContainer.LoadImages(ctx, projectImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load image into k3s")

	By("writing kubeconfig to temp file")
	kubeConfigYAML, err := k3sContainer.GetKubeConfig(ctx)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to get kubeconfig from k3s")
	tmpFile, err := os.CreateTemp("", "wmo-e2e-kubeconfig-*.yaml")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	_, err = tmpFile.Write(kubeConfigYAML)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, tmpFile.Close()).To(Succeed())
	kubeconfigFile = tmpFile.Name()
	Expect(os.Setenv("KUBECONFIG", kubeconfigFile)).To(Succeed())

	// By("installing CertManager")
	// Expect(utils.InstallCertManager()).To(Succeed(), "Failed to install CertManager")

	By("creating manager namespace")
	cmd = exec.Command("kubectl", "create", "ns", namespace)
	_, _ = utils.Run(cmd)

	By("labeling the namespace to enforce the restricted security policy")
	cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
		"pod-security.kubernetes.io/enforce=restricted")
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

	By("installing CRDs")
	cmd = exec.Command("just", "install")
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to install CRDs")

	By("deploying the controller-manager")
	cmd = exec.Command("just", fmt.Sprintf("img=%s", projectImage), "deploy")
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")

	By("setting test notification topic")
	cmd = exec.Command("kubectl", "set", "env", "deployment/warframe-market-operator-controller-manager",
		"NTFY_TOPIC=wmo--price-watch-test", "-n", namespace)
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to set NTFY_TOPIC")

	By("waiting for the controller-manager pod to be running")
	Eventually(func(g Gomega) {
		cmd := exec.Command("kubectl", "get",
			"pods", "-l", "control-plane=controller-manager",
			"-o", "go-template={{ range .items }}"+
				"{{ if not .metadata.deletionTimestamp }}"+
				"{{ .metadata.name }}"+
				"{{ \"\\n\" }}{{ end }}{{ end }}",
			"-n", namespace,
		)
		podOutput, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		podNames := utils.GetNonEmptyLines(podOutput)
		g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
		controllerPodName = podNames[0]

		cmd = exec.Command("kubectl", "get",
			"pods", controllerPodName,
			"-o", "jsonpath={.status.containerStatuses[0].ready}",
			"-n", namespace,
		)
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal("true"), "controller container not yet ready")
	}, 2*time.Minute, 5*time.Second).Should(Succeed())

	By("creating metrics ClusterRoleBinding")
	clusterRoleBindingYAML := fmt.Sprintf(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: warframe-market-operator-metrics-reader
subjects:
- kind: ServiceAccount
  name: %s
  namespace: %s
`, metricsRoleBindingName, serviceAccountName, namespace)
	cmd = exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(clusterRoleBindingYAML)
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to create metrics ClusterRoleBinding")
})

var _ = AfterSuite(func() {
	ctx := context.Background()

	By("cleaning up the curl pod for metrics")
	cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace, "--ignore-not-found")
	_, _ = utils.Run(cmd)

	By("cleaning up metrics ClusterRoleBinding")
	cmd = exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found")
	_, _ = utils.Run(cmd)

	By("undeploying the controller-manager")
	cmd = exec.Command("just", "undeploy")
	_, _ = utils.Run(cmd)

	By("uninstalling CRDs")
	cmd = exec.Command("just", "uninstall")
	_, _ = utils.Run(cmd)

	By("terminating k3s container")
	if k3sContainer != nil {
		_ = k3sContainer.Terminate(ctx)
	}

	By("removing kubeconfig temp file")
	if kubeconfigFile != "" {
		_ = os.Remove(kubeconfigFile)
	}
})
