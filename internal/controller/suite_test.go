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

package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kruize/kruize-operator/internal/utils"
	ctrl "sigs.k8s.io/controller-runtime"

	kruizev1alpha1 "github.com/kruize/kruize-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

// envtestK8sVersion is the Kubernetes control plane version whose binaries
// setup-envtest downloads. Must match ENVTEST_K8S_VERSION in the Makefile.
const envtestK8sVersion = "1.31.0"

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")

	// Lookup order: KUBEBUILDER_ASSETS env var → project-local bin/k8s/ → setup-envtest system cache.
	binaryAssetsDir := os.Getenv("KUBEBUILDER_ASSETS")
	if binaryAssetsDir == "" {
		pkgDir, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred(), "could not determine working directory")
		localDir := filepath.Join(pkgDir, "..", "..", "bin", "k8s",
			fmt.Sprintf("%s-%s-%s", envtestK8sVersion, runtime.GOOS, runtime.GOARCH))
		if _, err := os.Stat(localDir); err == nil {
			binaryAssetsDir = localDir
		}
	}
	if binaryAssetsDir == "" {
		systemDir, err := envtest.SetupEnvtestDefaultBinaryAssetsDirectory()
		Expect(err).NotTo(HaveOccurred(),
			"failed to resolve envtest binary assets directory from system cache; "+
				"run `make test` once to download envtest binaries for Kubernetes %s, "+
				"or set KUBEBUILDER_ASSETS to the directory containing etcd and kube-apiserver",
			envtestK8sVersion)
		binaryAssetsDir = filepath.Join(systemDir,
			fmt.Sprintf("%s-%s-%s", envtestK8sVersion, runtime.GOOS, runtime.GOARCH))
	}

	// Fail fast with a clear message if the resolved directory does not exist on disk.
	// Without this, testEnv.Start() would fail with a vague "fork/exec etcd: no such file" error.
	_, statErr := os.Stat(binaryAssetsDir)
	Expect(statErr).NotTo(HaveOccurred(),
		"envtest binary assets directory not found at %s; "+
			"run `make test` once to download envtest binaries for Kubernetes %s, "+
			"or set KUBEBUILDER_ASSETS to the directory containing etcd and kube-apiserver",
		binaryAssetsDir, envtestK8sVersion)

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: binaryAssetsDir,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = kruizev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// ADDED FOR NATIVE METRICS AUTH TESTING
	By("starting the manager with native metrics auth")
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: utils.GetMetricsOptions(utils.LocalMetricsAddr, true, false),
	})

	Expect(err).ToNot(HaveOccurred())
	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
