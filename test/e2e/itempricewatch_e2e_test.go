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
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/benkeil/warframe-market-operator/test/utils"
)

var _ = Describe("ItemPriceWatch", func() {
	const crName = "e2e-primed-firestorm"

	AfterEach(func() {
		cmd := exec.Command("kubectl", "delete", "itempricewatches", crName,
			"-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)
	})

	It("should reconcile and populate cheapestPrice in status", func() {
		By("creating an ItemPriceWatch CR")
		cr := fmt.Sprintf(`
apiVersion: warframe.market/v1alpha1
kind: ItemPriceWatch
metadata:
  name: %s
  namespace: %s
spec:
  itemSlug: primed_firestorm
  threshold: 9999
`, crName, namespace)
		cmd := exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(cr)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create ItemPriceWatch CR")

		By("waiting for cheapestPrice to be set in status")
		verifyCheapestPrice := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "itempricewatches", crName,
				"-n", namespace,
				"-o", "jsonpath={.status.cheapestPrice}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).NotTo(BeEmpty(), "cheapestPrice should be set")
			g.Expect(output).NotTo(Equal("0"), "cheapestPrice should be > 0")
		}
		Eventually(verifyCheapestPrice, 3*time.Minute, 10*time.Second).Should(Succeed())

		By("verifying the PriceSynced condition is True")
		verifySynced := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "itempricewatches", crName,
				"-n", namespace,
				"-o", `jsonpath={.status.conditions[?(@.type=="PriceSynced")].status}`)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("True"), "PriceSynced condition should be True")
		}
		Eventually(verifySynced, 3*time.Minute, 10*time.Second).Should(Succeed())

		By("verifying reconcile metrics for itempricewatch controller")
		metricsOutput := getMetricsOutput()
		Expect(metricsOutput).To(ContainSubstring(
			`controller_runtime_reconcile_total{controller="itempricewatch"`,
		))
	})
})
