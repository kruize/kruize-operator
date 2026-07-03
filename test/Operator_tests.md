# Operator tests Documentation

## Overview

This directory contains overview of tests for the Kruize Operator. Unit tests focus on testing individual functions and methods in isolation, ensuring correctness of core logic without requiring a full Kubernetes cluster.

## Test Structure

```
test/
├── Operator_tests.md           # Operator tests documentation
├── e2e/                        # End-to-end tests (see e2e_test.md)
│   ├── e2e_suite_test.go
│   └── e2e_test.go
│   └── e2e_test.md
└── utils/                      # Test utilities
    └── utils.go
```

## Unit Tests Location

Unit tests are co-located with the code they test:

```
internal/
└── controller/
    ├── kruize_controller.go        # Controller implementation
    ├── kruize_controller_test.go   # Unit tests
    └── suite_test.go               # Test suite setup
```

## Test Framework

We use [Ginkgo](https://onsi.github.io/ginkgo/) (BDD-style testing framework) and [Gomega](https://onsi.github.io/gomega/) (matcher library) for all tests.


## Test Types

### End-to-End (E2E) Tests

**Location**: `test/e2e/`

**Purpose**: Validate complete operator deployment and Kruize functionality on real Kubernetes clusters

**Documentation**: See [`e2e_tests_readme.md`](../e2e_test.md) for comprehensive e2e test documentation

**Quick Start**:
```bash
# Run on kind cluster (default)
go test ./test/e2e/... -v

# Run on minikube cluster
go test ./test/e2e/... -v -- -cluster-type=minikube
```

### Unit Tests

**Location**: `internal/controller/`

**Purpose**: Test individual functions and methods in isolation

**Documentation**: See [`Test_readme.md`](../Operator_tests.md) for unit test documentation

**Quick Start**:
```bash
# Run all unit tests
go test ./internal/controller/... -v

# Run with coverage
go test ./internal/controller/... -v -coverprofile=coverage.out
```
## Running Unit Tests

### Basic Commands

```bash
# Run all unit tests
go test ./internal/controller/... -v

# Run with coverage
go test ./internal/controller/... -v -coverprofile=coverage.out

# View coverage report
go tool cover -html=coverage.out

# Run specific test
go test ./internal/controller/... -v -ginkgo.focus="should generate RBAC"

# Run tests matching pattern
go test ./internal/controller/... -v -ginkgo.focus="OpenShift"
```

### Advanced Options

```bash
# Run with verbose Ginkgo output
go test ./internal/controller/... -v -ginkgo.v

# Run with trace (shows test execution flow)
go test ./internal/controller/... -v -ginkgo.trace

# Run in parallel
go test ./internal/controller/... -v -ginkgo.p
```


## Test Categories

### 1. Resource Generation Tests

**Purpose**: Verify correct generation of Kubernetes resources

**Tests**:
- RBAC manifests (Roles, RoleBindings, ServiceAccounts)
- Deployments (Kruize, Kruize-DB, Kruize-UI)
- Services (Kruize, Kruize-DB, Kruize-UI)
- Routes (OpenShift-specific)
- ConfigMaps

### 2. Cluster-Specific Behavior Tests

**Purpose**: Verify correct behavior for different cluster types

**Cluster Types**:
- **OpenShift**: Uses Routes, kruize-sa ServiceAccount
- **Kubernetes (kind/minikube)**: default ServiceAccount

### 3. Pod Specification Tests

**Purpose**: Verify correct pod configurations

**Tests**:
- **Container specifications**:
    - `should generate Kruize pod specification` - Verifies Kruize deployment container name and existence
    - `should generate Kruize-ui pod specification` - Verifies Kruize UI pod container configuration
    - `should generate Kruize-db pod specification` - Verifies Kruize DB deployment container name and existence


### 4. Service Configuration Tests

**Purpose**: Verify correct service configurations

**Tests**:
- Service types (ClusterIP, NodePort)
- Port configurations

### 5. Finalizer Lifecycle Tests

**Purpose**: Verify proper finalizer management and resource cleanup

**Tests**:
- Finalizer addition during CR creation
- Finalizer idempotency (no duplicates)
- Prevention of immediate deletion when finalizer present
- Finalizer removal after successful cleanup
- Cluster-scoped resource cleanup (OpenShift)
- Cluster-scoped resource cleanup (Kubernetes)
- Error handling during cleanup
- Cluster type validation before finalizer addition
- Full lifecycle for OpenShift cluster type
- Full lifecycle for Minikube cluster type
- Full lifecycle for Kind cluster type

**Finalizer Timeout Tests**:
- Default timeout configuration (30 seconds)
- Custom timeout from FINALIZER_TIMEOUT_SECONDS environment variable
- Fallback to default on invalid environment variable values
- Timeout detection for slow operations
- Fast operations complete within timeout

### 6. Test Mode Behavior Tests

**Purpose**: Verify test mode functionality for faster test execution

**Tests**:
- Test mode detection via environment variable
- Pod readiness check bypass in test mode

### 7. Metrics Authentication Tests

**Purpose**: Verify metrics endpoint security

**Tests**:
- Unauthorized request rejection
- Invalid token validation
- Metrics server availability

## Resources

- [Ginkgo Documentation](https://onsi.github.io/ginkgo/)
- [Gomega Matchers](https://onsi.github.io/gomega/)
- [Controller Runtime Testing](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest)
- [Operator SDK Testing Guide](https://sdk.operatorframework.io/docs/building-operators/golang/testing/)
