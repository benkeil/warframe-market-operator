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

var _ = Describe("RivenPriceWatch", func() {
	const crName = "e2e-falcor-riven"

	AfterEach(func() {
		cmd := exec.Command("kubectl", "delete", "rivenpricewatches", crName,
			"-n", namespace, "--ignore-not-found")
		_, _ = utils.Run(cmd)
	})

	It("should reconcile and set a condition in status", func() {
		By("creating a RivenPriceWatch CR")
		cr := fmt.Sprintf(`
apiVersion: warframe.market/v1alpha1
kind: RivenPriceWatch
metadata:
  name: %s
  namespace: %s
spec:
  weaponSlug: falcor
  positiveStats:
    - critical_chance
  threshold: 9999
  playerStatus:
    - ingame
    - online
    - offline
`, crName, namespace)
		cmd := exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(cr)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create RivenPriceWatch CR")

		By("waiting for PriceSynced condition to be set")
		verifyCondition := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "rivenpricewatches", crName,
				"-n", namespace,
				"-o", `jsonpath={.status.conditions[?(@.type=="PriceSynced")].type}`)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("PriceSynced"), "PriceSynced condition should be present")
		}
		Eventually(verifyCondition, 3*time.Minute, 10*time.Second).Should(Succeed())

		By("verifying reconcile metrics for rivenpricewatch controller")
		metricsOutput := getMetricsOutput()
		Expect(metricsOutput).To(ContainSubstring(
			`controller_runtime_reconcile_total{controller="rivenpricewatch"`,
		))
	})
})
