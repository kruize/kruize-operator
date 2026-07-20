/*
Copyright 2025.

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

package constants

import "os"

// Environment variable names for image overrides
const (
	// envAutotuneImage is the environment variable name for overriding the default Autotune image
	envAutotuneImage = "DEFAULT_AUTOTUNE_IMAGE"
	
	// envUIImage is the environment variable name for overriding the default Kruize UI image
	envUIImage = "DEFAULT_AUTOTUNE_UI_IMAGE"
	
	// envOptimizerImage is the environment variable name for overriding the default Optimizer image
	envOptimizerImage = "DEFAULT_OPTIMIZER_IMAGE"
)

// Default container image versions
const (
	// defaultAutotuneImageTag is the default tag for Kruize Autotune image
	defaultAutotuneImageTag = "0.11.1"
	
	// defaultUIImageTag is the default tag for Kruize UI image
	defaultUIImageTag = "0.1.0"
	
	// defaultOptimizerImageTag is the default tag for Kruize Optimizer image
	defaultOptimizerImageTag = "0.0.1"
	
	// defaultAutotuneImageRepo is the default repository for Kruize Autotune image
	defaultAutotuneImageRepo = "quay.io/kruize/autotune_operator"
	
	// defaultUIImageRepo is the default repository for Kruize UI image
	defaultUIImageRepo = "quay.io/kruize/kruize-ui"
	
	// defaultOptimizerImageRepo is the default repository for Kruize Optimizer image
	defaultOptimizerImageRepo = "quay.io/kruize/kruize-optimizer"
)

// Package-level variables that cache the resolved default images.
// These are initialized once at package load time to avoid repeated
// environment variable lookups on every function call.
var (
	defaultAutotuneImage   string
	defaultUIImage         string
	defaultOptimizerImage  string
)

// init resolves the default images by checking environment variables once at package initialization.
// This caching approach improves performance and makes behavior more predictable.
func init() {
	// Resolve Autotune image
	if envImage := os.Getenv(envAutotuneImage); envImage != "" {
		defaultAutotuneImage = envImage
	} else {
		defaultAutotuneImage = defaultAutotuneImageRepo + ":" + defaultAutotuneImageTag
	}
	
	// Resolve Autotune UI image
	if envImage := os.Getenv(envUIImage); envImage != "" {
		defaultUIImage = envImage
	} else {
		defaultUIImage = defaultUIImageRepo + ":" + defaultUIImageTag
	}
	
	// Resolve Optimizer image
	if envImage := os.Getenv(envOptimizerImage); envImage != "" {
		defaultOptimizerImage = envImage
	} else {
		defaultOptimizerImage = defaultOptimizerImageRepo + ":" + defaultOptimizerImageTag
	}
}

// GetDefaultAutotuneImage returns the cached default Autotune image.
// The image is resolved once at package initialization from environment variables or defaults.
func GetDefaultAutotuneImage() string {
	return defaultAutotuneImage
}

// GetDefaultUIImage returns the cached default Autotune UI image.
// The image is resolved once at package initialization from environment variables or defaults.
func GetDefaultUIImage() string {
	return defaultUIImage
}

// GetDefaultOptimizerImage returns the cached default Optimizer image.
// The image is resolved once at package initialization from environment variables or defaults.
func GetDefaultOptimizerImage() string {
	return defaultOptimizerImage
}
