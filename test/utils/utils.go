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

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kruize/kruize-operator/internal/constants"
	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive
	"gopkg.in/yaml.v3"
)

const (
	kubePrometheusRepo = "https://github.com/prometheus-operator/kube-prometheus.git"
	kubePrometheusDir  = "kube-prometheus"
	cadvisorRepo       = "https://github.com/google/cadvisor.git"
	cadvisorDir        = "cadvisor"

	certmanagerVersion = "v1.14.4"
	certmanagerURLTmpl = "https://github.com/jetstack/cert-manager/releases/download/%s/cert-manager.yaml"
)

func warnError(err error) {
	fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// getKubePrometheusVersion returns the appropriate kube-prometheus version based on cluster type
func getKubePrometheusVersion(clusterType string) string {
	return constants.GetKubePrometheusVersion(clusterType)
}

// installCadvisor installs cadvisor for monitoring container metrics
func installCadvisor() error {
	// Check if cadvisor daemonset already exists
	checkCmd := exec.Command("kubectl", "get", "daemonset", "cadvisor", "-n", "cadvisor")
	_, err := Run(checkCmd)
	if err == nil {
		fmt.Fprintf(GinkgoWriter, "cadvisor already installed, skipping installation\n")
		return nil
	}

	// Clone cadvisor repository
	fmt.Fprintf(GinkgoWriter, "Cloning cadvisor repository...\n")
	cloneCmd := exec.Command("git", "clone", cadvisorRepo)
	_, err = Run(cloneCmd)
	if err != nil {
		// Check if directory already exists
		if _, statErr := os.Stat(cadvisorDir); statErr == nil {
			fmt.Fprintf(GinkgoWriter, "cadvisor directory already exists, using existing clone\n")
		} else {
			return fmt.Errorf("failed to clone cadvisor: %w", err)
		}
	}

	// Apply cadvisor manifests using kustomize
	fmt.Fprintf(GinkgoWriter, "Applying cadvisor manifests...\n")
	cadvisorBasePath := fmt.Sprintf("%s/deploy/kubernetes/base", cadvisorDir)

	// Use kubectl kustomize to build and apply
	kustomizeCmd := exec.Command("kubectl", "kustomize", cadvisorBasePath)
	kustomizeOutput, err := Run(kustomizeCmd)
	if err != nil {
		return fmt.Errorf("failed to kustomize cadvisor manifests: %w", err)
	}

	// Apply the kustomized output
	applyCmd := exec.Command("kubectl", "apply", "-f", "-")
	applyCmd.Stdin = strings.NewReader(string(kustomizeOutput))
	_, err = Run(applyCmd)
	if err != nil {
		return fmt.Errorf("failed to apply cadvisor manifests: %w", err)
	}

	// Wait for cadvisor daemonset to be ready
	fmt.Fprintf(GinkgoWriter, "Waiting for cadvisor daemonset to be ready...\n")
	waitCmd := exec.Command("kubectl", "rollout", "status", "daemonset/cadvisor", "-n", "cadvisor", "--timeout=120s")
	_, err = Run(waitCmd)
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "Warning: cadvisor rollout status check failed, continuing: %v\n", err)
	}

	fmt.Fprintf(GinkgoWriter, "cadvisor installation completed successfully\n")
	return nil
}

// InstallPrometheusOperator installs kube-prometheus stack (includes Prometheus Operator, Prometheus, Grafana, etc.)
// This installs in the monitoring namespace by default
// clusterType: "kind" or "minikube" - determines which kube-prometheus version to use
func InstallPrometheusOperator(clusterType string) error {
	prometheusNS := "monitoring"

	// Check if prometheus-k8s statefulset already exists in monitoring namespace
	checkCmd := exec.Command("kubectl", "get", "statefulset", "prometheus-k8s", "-n", prometheusNS)
	_, err := Run(checkCmd)
	if err == nil {
		fmt.Fprintf(GinkgoWriter, "kube-prometheus already installed in %s namespace, skipping installation\n", prometheusNS)
		return nil
	}

	// Get version based on cluster type
	kubePrometheusVersion := getKubePrometheusVersion(clusterType)
	fmt.Fprintf(GinkgoWriter, "Installing kube-prometheus %s for %s cluster in %s namespace\n", kubePrometheusVersion, clusterType, prometheusNS)

	// Step 1: Install cadvisor
	fmt.Fprintf(GinkgoWriter, "Installing cadvisor...\n")
	if err := installCadvisor(); err != nil {
		return fmt.Errorf("failed to install cadvisor: %w", err)
	}

	// Step 2: Clone kube-prometheus repository
	fmt.Fprintf(GinkgoWriter, "Cloning kube-prometheus repository...\n")
	cloneCmd := exec.Command("git", "clone", "-b", kubePrometheusVersion, kubePrometheusRepo)
	_, err = Run(cloneCmd)
	if err != nil {
		// Check if directory already exists
		if _, statErr := os.Stat(kubePrometheusDir); statErr == nil {
			fmt.Fprintf(GinkgoWriter, "kube-prometheus directory already exists, using existing clone\n")
		} else {
			return fmt.Errorf("failed to clone kube-prometheus: %w", err)
		}
	}

	manifestsPath := fmt.Sprintf("%s/manifests", kubePrometheusDir)

	// Step 3: Install CRDs and namespace using server-side apply
	fmt.Fprintf(GinkgoWriter, "Installing CRDs with server-side apply...\n")
	setupPath := fmt.Sprintf("%s/setup", manifestsPath)
	cmd := exec.Command("kubectl", "apply", "-f", setupPath, "--server-side")
	output, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to install kube-prometheus CRDs: %w", err)
	}
	fmt.Fprintf(GinkgoWriter, "CRDs installation output: %s\n", string(output))

	// Step 4: Wait for CRDs to be established
	fmt.Fprintf(GinkgoWriter, "Waiting for CRDs to be established...\n")
	crds := []string{
		"servicemonitors.monitoring.coreos.com",
		"prometheuses.monitoring.coreos.com",
		"alertmanagers.monitoring.coreos.com",
		"prometheusrules.monitoring.coreos.com",
	}

	for _, crd := range crds {
		// First check if CRD exists
		checkCRDCmd := exec.Command("kubectl", "get", "crd", crd)
		_, err = Run(checkCRDCmd)
		if err != nil {
			return fmt.Errorf("CRD %s not found: %w", crd, err)
		}

		// Wait for it to be established
		waitCmd := exec.Command("kubectl", "wait", "--for", "condition=Established", "--timeout=60s", "crd/"+crd)
		_, err = Run(waitCmd)
		if err != nil {
			return fmt.Errorf("failed waiting for CRD %s to be established: %w", crd, err)
		}
	}

	// Step 5: Install kube-prometheus manifests using server-side apply
	fmt.Fprintf(GinkgoWriter, "Installing kube-prometheus manifests with server-side apply...\n")
	cmd = exec.Command("kubectl", "apply", "-f", manifestsPath, "--server-side")
	output, err = Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to install kube-prometheus manifests: %w", err)
	}
	fmt.Fprintf(GinkgoWriter, "Manifests installation output: %s\n", string(output))

	// Step 6: Wait for monitoring namespace to be ready
	fmt.Fprintf(GinkgoWriter, "Waiting for monitoring namespace to be ready...\n")
	waitNSCmd := exec.Command("kubectl", "wait", "--for=jsonpath={.status.phase}=Active", "--timeout=60s", "namespace/"+prometheusNS)
	_, err = Run(waitNSCmd)
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "Warning: namespace wait failed, continuing: %v\n", err)
	}

	// Wait for prometheus-operator deployment to be ready first
	fmt.Fprintf(GinkgoWriter, "Waiting for prometheus-operator deployment to be ready...\n")
	waitOpCmd := exec.Command("kubectl", "wait", "--for=condition=Available", "--timeout=120s", "deployment/prometheus-operator", "-n", prometheusNS)
	_, err = Run(waitOpCmd)
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "Warning: prometheus-operator wait failed, continuing: %v\n", err)
	}

	// Wait for prometheus-k8s statefulset to be ready
	fmt.Fprintf(GinkgoWriter, "Waiting for Prometheus statefulset to be ready...\n")
	// Use kubectl wait with polling for statefulset readiness
	waitPrometheusCmd := exec.Command("kubectl", "wait", "--for=jsonpath={.status.readyReplicas}=2", "--timeout=300s", "statefulset/prometheus-k8s", "-n", prometheusNS)
	_, err = Run(waitPrometheusCmd)
	if err != nil {
		// Fallback: check if at least one replica is ready
		fmt.Fprintf(GinkgoWriter, "Full readiness check failed, checking for partial readiness...\n")
		checkCmd := exec.Command("kubectl", "get", "statefulset", "prometheus-k8s", "-n", prometheusNS, "-o", "jsonpath={.status.readyReplicas}")
		output, checkErr := Run(checkCmd)
		if checkErr == nil && len(output) > 0 && string(output) != "0" && string(output) != "" {
			fmt.Fprintf(GinkgoWriter, "Prometheus partially ready with %s replicas, continuing...\n", string(output))
		} else {
			return fmt.Errorf("timeout waiting for Prometheus pods to be ready: %w", err)
		}
	} else {
		fmt.Fprintf(GinkgoWriter, "Prometheus statefulset is ready\n")
	}

	fmt.Fprintf(GinkgoWriter, "kube-prometheus installation completed successfully\n")
	return nil
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) ([]byte, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		fmt.Fprintf(GinkgoWriter, "chdir dir: %s\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	fmt.Fprintf(GinkgoWriter, "running: %s\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s failed with error: (%v) %s", command, err, string(output))
	}

	return output, nil
}

// UninstallPrometheusOperator uninstalls kube-prometheus stack
// Only uninstalls if it was installed by this test run
func UninstallPrometheusOperator() {
	prometheusNS := "monitoring"

	// Check if kube-prometheus is actually installed
	checkCmd := exec.Command("kubectl", "get", "statefulset", "prometheus-k8s", "-n", prometheusNS)
	_, err := Run(checkCmd)
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "kube-prometheus not found, skipping uninstallation\n")
		// Still clean up the cloned directory if it exists
		if err := os.RemoveAll(kubePrometheusDir); err != nil {
			fmt.Fprintf(GinkgoWriter, "Warning: failed to remove %s directory: %v\n", kubePrometheusDir, err)
		}
		return
	}

	fmt.Fprintf(GinkgoWriter, "Uninstalling kube-prometheus\n")

	manifestsPath := fmt.Sprintf("%s/manifests", kubePrometheusDir)
	setupPath := fmt.Sprintf("%s/setup", manifestsPath)

	// Check if manifests directory exists before trying to delete
	if _, err := os.Stat(manifestsPath); err == nil {
		// Delete manifests first
		cmd := exec.Command("kubectl", "delete", "--ignore-not-found=true", "-f", manifestsPath)
		if _, err := Run(cmd); err != nil {
			warnError(err)
		}

		// Delete CRDs and namespace (setup)
		cmd = exec.Command("kubectl", "delete", "--ignore-not-found=true", "-f", setupPath)
		if _, err := Run(cmd); err != nil {
			warnError(err)
		}
	} else {
		fmt.Fprintf(GinkgoWriter, "Manifests directory not found, skipping resource deletion\n")
	}

	// Clean up cloned directory
	if err := os.RemoveAll(kubePrometheusDir); err != nil {
		fmt.Fprintf(GinkgoWriter, "Warning: failed to remove %s directory: %v\n", kubePrometheusDir, err)
	}

	// Uninstall cadvisor
	uninstallCadvisor()

	fmt.Fprintf(GinkgoWriter, "kube-prometheus uninstallation completed\n")
}

// uninstallCadvisor uninstalls cadvisor
func uninstallCadvisor() {
	fmt.Fprintf(GinkgoWriter, "Uninstalling cadvisor\n")

	cadvisorBasePath := fmt.Sprintf("%s/deploy/kubernetes/base", cadvisorDir)

	// Check if cadvisor manifests directory exists before trying to delete
	if _, err := os.Stat(cadvisorBasePath); err == nil {
		// Use kubectl kustomize to build and delete
		kustomizeCmd := exec.Command("kubectl", "kustomize", cadvisorBasePath)
		kustomizeOutput, err := Run(kustomizeCmd)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Warning: failed to kustomize cadvisor manifests for deletion: %v\n", err)
		} else {
			// Delete the kustomized output
			deleteCmd := exec.Command("kubectl", "delete", "--ignore-not-found=true", "-f", "-")
			deleteCmd.Stdin = strings.NewReader(string(kustomizeOutput))
			if _, err := Run(deleteCmd); err != nil {
				warnError(err)
			}
		}
	} else {
		fmt.Fprintf(GinkgoWriter, "cadvisor manifests directory not found, skipping resource deletion\n")
	}

	// Clean up cloned directory
	if err := os.RemoveAll(cadvisorDir); err != nil {
		fmt.Fprintf(GinkgoWriter, "Warning: failed to remove %s directory: %v\n", cadvisorDir, err)
	}

	fmt.Fprintf(GinkgoWriter, "cadvisor uninstallation completed\n")
}

// UninstallCertManager uninstalls the cert manager
func UninstallCertManager() {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// InstallCertManager installs the cert manager bundle.
func InstallCertManager() error {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "apply", "-f", url)
	if _, err := Run(cmd); err != nil {
		return err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = exec.Command("kubectl", "wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "5m",
	)

	_, err := Run(cmd)
	return err
}

// LoadImageToKindCluster loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	cmd := exec.Command("kind", kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.Replace(wd, "/test/e2e", "", -1)
	return wd, nil
}

// UpdateKruizeSampleYAML creates a temporary copy of v1alpha1_kruize.yaml with the specified cluster type, namespace, and optional images
// Returns the path to the temporary file, which should be cleaned up by the caller
func UpdateKruizeSampleYAML(clusterType, namespace, kruizeImage, kruizeUIImage string) (string, error) {
	sourcePath := "config/samples/v1alpha1_kruize.yaml"

	// Read the original file
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to read sample YAML: %w", err)
	}

	// Parse YAML into a generic map structure
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(content, &yamlData); err != nil {
		return "", fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Navigate to the spec section and update fields
	spec, ok := yamlData["spec"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid YAML structure: 'spec' field not found or not a map")
	}

	// Update cluster_type
	spec["cluster_type"] = clusterType

	// Update namespace
	spec["namespace"] = namespace

	// Update autotune_image if specified
	if kruizeImage != "" {
		spec["autotune_image"] = kruizeImage
		fmt.Fprintf(GinkgoWriter, "Updated autotune_image to %s\n", kruizeImage)
	}

	// Update autotune_ui_image if specified
	if kruizeUIImage != "" {
		spec["autotune_ui_image"] = kruizeUIImage
		fmt.Fprintf(GinkgoWriter, "Updated autotune_ui_image to %s\n", kruizeUIImage)
	}

	// Marshal the updated YAML back to bytes
	updatedContent, err := yaml.Marshal(&yamlData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal updated YAML: %w", err)
	}

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "kruize-sample-*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer tmpFile.Close()

	// Write the modified content to the temporary file
	if _, err := tmpFile.Write(updatedContent); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write to temporary file: %w", err)
	}

	tmpPath := tmpFile.Name()
	fmt.Fprintf(GinkgoWriter, "Created temporary sample CR at %s with cluster_type=%s, namespace=%s\n", tmpPath, clusterType, namespace)
	return tmpPath, nil
}

// CleanupTempFile removes a temporary file created by UpdateKruizeSampleYAML
func CleanupTempFile(path string) {
	if path != "" && filepath.Base(path) != "v1alpha1_kruize.yaml" {
		if err := os.Remove(path); err != nil {
			fmt.Fprintf(GinkgoWriter, "Warning: failed to remove temporary file %s: %v\n", path, err)
		} else {
			fmt.Fprintf(GinkgoWriter, "Cleaned up temporary file %s\n", path)
		}
	}
}

// ExtractImageFromMakefile extracts the operator image from Makefile
func ExtractImageFromMakefile() (string, error) {
	projectDir, err := GetProjectDir()
	if err != nil {
		return "", fmt.Errorf("failed to get project directory: %w", err)
	}

	makefilePath := filepath.Join(projectDir, "Makefile")
	content, err := os.ReadFile(makefilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read Makefile: %w", err)
	}

	// Extract IMAGE_TAG_BASE and VERSION
	// Use [ \t]* to only match spaces/tabs, not newlines
	imageTagBaseRe := regexp.MustCompile(`(?m)^IMAGE_TAG_BASE[ \t]*\?=[ \t]*([^\n\r]*)$`)
	versionRe := regexp.MustCompile(`(?m)^VERSION[ \t]*\?=[ \t]*([^\n\r]*)$`)

	imageTagBaseMatch := imageTagBaseRe.FindStringSubmatch(string(content))
	versionMatch := versionRe.FindStringSubmatch(string(content))

	if len(imageTagBaseMatch) < 2 || len(versionMatch) < 2 {
		return "", fmt.Errorf("failed to extract IMAGE_TAG_BASE or VERSION from Makefile")
	}

	imageTagBase := strings.TrimSpace(imageTagBaseMatch[1])
	version := strings.TrimSpace(versionMatch[1])

	if imageTagBase == "" {
		return "", fmt.Errorf("IMAGE_TAG_BASE is empty in Makefile")
	}

	if version == "" {
		return "", fmt.Errorf("VERSION is empty in Makefile")
	}

	return fmt.Sprintf("%s:%s", imageTagBase, version), nil
}
