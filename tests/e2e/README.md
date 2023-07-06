# End-to-End Testing

## Context

The FSx OpenZFS CSI driver utilizes a variety of workflows to validate end to end functionality in a Kubernetes environment.
The framework and existing testsuites for Kubernetes storage E2E testing is leveraged to test common cases.
Untested or unique test cases are also tested using a custom testsuite.

Due to the nature of end-to-end tests it is possible to leave testing artifacts stranded.
Events such as abrupt termination or server-side API failures can prevent resources from being deleted properly.
The `fsx` hack contains a function that cleans up all testing resources that may have been left stranded.
Run `bash ../../hack/e2e/fsx.sh fsx_delete_e2e_resources` to execute this function.

## Running

In order to run the E2E tests, an EKS cluster with an OpenZFS driver installed must be utilized.
Follow the respective section depending on if a test environment is already set up.

### Existing Environment

The test environment should contain the following:
1. Kubernetes 1.13+ (CSI 1.0).
2. EKS Cluster
3. Ginkgo CLI

Run `ginkgo -r --procs=4 --timeout=5h --junit-report=report.xml` from the tests/e2e folder to run