/*
Copyright 2024.

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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kruize/kruize-operator/internal/constants"
	"github.com/kruize/kruize-operator/test/utils"
)

var _ = Describe("controller", Ordered, func() {
	BeforeAll(func() {
		// Skip Prometheus Operator installation on OpenShift as it's pre-installed
		if clusterType != constants.ClusterTypeOpenShift {
			By(fmt.Sprintf("installing prometheus operator for %s cluster", clusterType))
			err := utils.InstallPrometheusOperator(clusterType)
			if err != nil {
				// Log the error but don't fail - Prometheus might already be installed
				fmt.Fprintf(GinkgoWriter, "Warning: Prometheus installation encountered an issue: %v\n", err)
				fmt.Fprintf(GinkgoWriter, "Continuing with tests - Prometheus may already be installed\n")
			}
		} else {
			By("skipping prometheus operator installation on OpenShift (pre-installed)")
		}

		By(fmt.Sprintf("creating namespace %s for operator and Kruize", namespace))
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	AfterAll(func() {
		// Collect pod logs before cleanup
		By("collecting pod logs before cleanup")

		// Define log directory path
		logDir := "/tmp/pod-logs"

		// Remove any existing directory/file/symlink at the target path
		// This prevents symlink attacks by ensuring we create a real directory
		_ = os.RemoveAll(logDir)

		// Create directory with secure permissions
		if err := os.Mkdir(logDir, 0700); err != nil {
			fmt.Fprintf(GinkgoWriter, "Warning: Failed to create log directory: %v\n", err)
			return
		}

		fmt.Fprintf(GinkgoWriter, "Collecting pod logs to %s\n", logDir)

		// Collect pod list
		cmd := exec.Command("kubectl", "get", "pods", "-n", namespace, "-o", "wide")
		if output, err := utils.Run(cmd); err != nil {
			fmt.Fprintf(GinkgoWriter, "Warning: Failed to collect pod list: %v\n", err)
		} else {
			if err := os.WriteFile(filepath.Join(logDir, "pod-list.txt"), output, 0600); err != nil {
				fmt.Fprintf(GinkgoWriter, "Warning: Failed to write pod list: %v\n", err)
			}
		}

		// Collect operator logs
		cmd = exec.Command("kubectl", "logs", "-n", namespace, "-l", "control-plane=kruize-operator", "--all-containers=true", "--prefix=true", "--tail=-1")
		if output, err := utils.Run(cmd); err != nil {
			fmt.Fprintf(GinkgoWriter, "Warning: Failed to collect operator logs: %v\n", err)
		} else {
			if err := os.WriteFile(filepath.Join(logDir, "operator-logs.txt"), output, 0600); err != nil {
				fmt.Fprintf(GinkgoWriter, "Warning: Failed to write operator logs: %v\n", err)
			}
		}

		// Collect operator pod description
		cmd = exec.Command("kubectl", "describe", "pods", "-n", namespace, "-l", "control-plane=kruize-operator")
		if output, err := utils.Run(cmd); err != nil {
			fmt.Fprintf(GinkgoWriter, "Warning: Failed to collect operator pod description: %v\n", err)
		} else {
			if err := os.WriteFile(filepath.Join(logDir, "operator-pod-description.txt"), output, 0600); err != nil {
				fmt.Fprintf(GinkgoWriter, "Warning: Failed to write operator pod description: %v\n", err)
			}
		}

		// Collect Kruize logs
		cmd = exec.Command("kubectl", "logs", "-n", namespace, "-l", "app=kruize", "--all-containers=true", "--prefix=true", "--tail=-1")
		if output, err := utils.Run(cmd); err != nil {
			fmt.Fprintf(GinkgoWriter, "Warning: Failed to collect Kruize logs: %v\n", err)
		} else {
			if err := os.WriteFile(filepath.Join(logDir, "kruize-logs.txt"), output, 0600); err != nil {
				fmt.Fprintf(GinkgoWriter, "Warning: Failed to write Kruize logs: %v\n", err)
			}
		}

		// Collect Kruize DB logs
		cmd = exec.Command("kubectl", "logs", "-n", namespace, "-l", "app=kruize-db", "--all-containers=true", "--prefix=true", "--tail=-1")
		if output, err := utils.Run(cmd); err != nil {
			fmt.Fprintf(GinkgoWriter, "Warning: Failed to collect Kruize DB logs: %v\n", err)
		} else {
			if err := os.WriteFile(filepath.Join(logDir, "kruize-db-logs.txt"), output, 0600); err != nil {
				fmt.Fprintf(GinkgoWriter, "Warning: Failed to write Kruize DB logs: %v\n", err)
			}
		}

		fmt.Fprintf(GinkgoWriter, "Pod logs collection completed\n")

		By("undeploying the kruize-operator")
		// Determine overlay based on cluster type
		overlay := "local"
		if clusterType == constants.ClusterTypeOpenShift {
			overlay = "openshift"
		}
		cmd = exec.Command("make", "undeploy", fmt.Sprintf("OVERLAY=%s", overlay))
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		// Skip Prometheus Operator uninstallation on OpenShift as it's pre-installed
		if clusterType != constants.ClusterTypeOpenShift {
			By("uninstalling prometheus operator")
			utils.UninstallPrometheusOperator()
		} else {
			By("skipping prometheus operator uninstallation on OpenShift (pre-installed)")
		}
	})

	Context("Operator", func() {
		It("should run successfully", func() {
			var controllerPodName string
			var err error

			By(fmt.Sprintf("using operator image: %s", operatorImage))

			By("installing CRDs")
			cmd := exec.Command("make", "install")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By(fmt.Sprintf("deploying the kruize-operator to namespace %s", namespace))
			// Determine overlay based on cluster type
			overlay := "local"
			if clusterType == constants.ClusterTypeOpenShift {
				overlay = "openshift"
			}
			cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", operatorImage), fmt.Sprintf("OVERLAY=%s", overlay))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("validating that the kruize-operator pod is running as expected")
			verifyControllerUp := func() error {
				// Get pod name
				cmd = exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=kruize-operator",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				if len(podNames) != 1 {
					return fmt.Errorf("expect 1 controller pods running, but got %d", len(podNames))
				}
				controllerPodName = podNames[0]
				ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("kruize-operator"))

				// Validate pod status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				status, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("controller pod in %s status", status)
				}
				return nil
			}
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, 10*time.Second).Should(Succeed())

		})

		It("should deploy Kruize components successfully", func() {
			var tmpCRPath string
			var err error

			By("creating a temporary Kruize custom resource")
			tmpCRPath, err = utils.UpdateKruizeSampleYAML(clusterType, namespace, kruizeImage, kruizeUIImage)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
			defer utils.CleanupTempFile(tmpCRPath)

			By("applying the Kruize custom resource")
			cmd := exec.Command("kubectl", "apply", "-f", tmpCRPath, "-n", namespace)
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("checking that kruize namespace is created")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "namespace", namespace)
				_, err := utils.Run(cmd)
				return err
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			// Determine expected service account based on cluster type
			expectedSA := "default"
			if clusterType == constants.ClusterTypeOpenShift {
				expectedSA = "kruize-sa"
			}

			By(fmt.Sprintf("checking that Kruize ServiceAccount '%s' is created", expectedSA))
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "serviceaccount", expectedSA, "-n", namespace)
				_, err := utils.Run(cmd)
				return err
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("checking that Kruize database is ready")
			Eventually(func() error {
				// Try kruize-db-deployment first, then kruize-db
				var cmd *exec.Cmd
				var output []byte
				var err error

				cmd = exec.Command("kubectl", "get", "deployment", "kruize-db-deployment", "-n", namespace, "-o", "jsonpath={.status.readyReplicas}")
				output, err = utils.Run(cmd)
				if err != nil {
					// Try kruize-db if kruize-db-deployment doesn't exist
					cmd = exec.Command("kubectl", "get", "deployment", "kruize-db", "-n", namespace, "-o", "jsonpath={.status.readyReplicas}")
					output, err = utils.Run(cmd)
					if err != nil {
						return fmt.Errorf("neither kruize-db-deployment nor kruize-db found: %w", err)
					}
				}

				if string(output) != "1" {
					return fmt.Errorf("database deployment not ready")
				}
				return nil
			}, 3*time.Minute, 10*time.Second).Should(Succeed())

			By("checking that Kruize deployment is ready")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "deployment", "kruize", "-n", namespace, "-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				if err != nil {
					return err
				}
				if string(output) != "1" {
					return fmt.Errorf("deployment not ready")
				}
				return nil
			}, 3*time.Minute, 10*time.Second).Should(Succeed())

			By("checking that Kruize UI deployment is ready")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "deployment", "kruize-ui-nginx", "-n", namespace, "-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				if err != nil {
					return err
				}
				if string(output) != "1" {
					return fmt.Errorf("UI deployment not ready")
				}
				return nil
			}, 3*time.Minute, 10*time.Second).Should(Succeed())

			By("verifying deployed Kruize image")
			cmd = exec.Command("kubectl", "get", "deployment", "kruize", "-n", namespace, "-o", "jsonpath={.spec.template.spec.containers[0].image}")
			output, err := utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
			deployedImage := strings.TrimSpace(string(output))
			fmt.Fprintf(GinkgoWriter, "Deployed Kruize image: %s\n", deployedImage)

			// If custom Kruize image was specified, verify it matches
			if kruizeImage != "" {
				if deployedImage == kruizeImage {
					fmt.Fprintf(GinkgoWriter, "✓ Deployed image matches specified KRUIZE_IMAGE: %s\n", kruizeImage)
				} else {
					fmt.Fprintf(GinkgoWriter, "⚠ Warning: Deployed image %s does not match specified KRUIZE_IMAGE %s\n", deployedImage, kruizeImage)
				}
			} else {
				fmt.Fprintf(GinkgoWriter, "Using default Kruize image from CR: %s\n", deployedImage)
			}

			By("verifying deployed Kruize UI image")
			cmd = exec.Command("kubectl", "get", "deployment", "kruize-ui-nginx", "-n", namespace, "-o", "jsonpath={.spec.template.spec.containers[0].image}")
			output, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
			deployedUIImage := strings.TrimSpace(string(output))
			fmt.Fprintf(GinkgoWriter, "Deployed Kruize UI image: %s\n", deployedUIImage)

			// If custom Kruize UI image was specified, verify it matches and fail on mismatch
			if kruizeUIImage != "" {
				fmt.Fprintf(GinkgoWriter, "Validating deployed UI image against KRUIZE_UI_IMAGE: %s\n", kruizeUIImage)
				ExpectWithOffset(1, deployedUIImage).To(
					Equal(kruizeUIImage),
					fmt.Sprintf("Deployed UI image %s does not match KRUIZE_UI_IMAGE %s", deployedUIImage, kruizeUIImage),
				)
				fmt.Fprintf(GinkgoWriter, "✓ Deployed UI image matches specified KRUIZE_UI_IMAGE: %s\n", kruizeUIImage)
			} else {
				fmt.Fprintf(GinkgoWriter, "Using default Kruize UI image from CR: %s\n", deployedUIImage)
			}

			By("verifying Kruize API is responding")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "exec", "deployment/kruize", "-n", namespace, "--", "curl", "-s", "localhost:8080/health")
				_, err := utils.Run(cmd)
				return err
			}, 3*time.Minute, 10*time.Second).Should(Succeed())

			By("checking that Kruize Optimizer deployment is ready")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "deployment", "kruize-optimizer", "-n", namespace, "-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				if err != nil {
					return err
				}
				if string(output) != "1" {
					return fmt.Errorf("kruize-optimizer deployment not ready")
				}
				return nil
			}, 3*time.Minute, 10*time.Second).Should(Succeed())

			By("checking that datasource is configured via Kruize API")
			var datasourceOutput string
			Eventually(func() error {
				cmd := exec.Command("kubectl", "exec", "deployment/kruize", "-n", namespace, "--", "curl", "-s", "localhost:8080/datasources")
				output, err := utils.Run(cmd)
				if err != nil {
					return err
				}

				datasourceOutput = string(output)

				// Check for prometheus-1 datasource
				if !strings.Contains(datasourceOutput, "prometheus-1") {
					return fmt.Errorf("prometheus-1 datasource not found in API response")
				}
				return nil
			}, 3*time.Minute, 20*time.Second).Should(Succeed())

			By("printing datasources API output")
			fmt.Fprintf(GinkgoWriter, "\n=== Datasources API Output ===\n%s\n==============================\n", datasourceOutput)

		})
	})
})
