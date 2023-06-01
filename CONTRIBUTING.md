# Contribution Guidelines

## Sign the CLA

Kubernetes projects require that you sign a Contributor License Agreement (CLA) before we can accept your pull requests.  
Please see https://git.k8s.io/community/CLA.md for more info.

## Contributing a Patch

1. Submit an issue describing your proposed change to the repo in question.
1. The [repo owners](OWNERS) will respond to your issue promptly.
1. If your proposed change is accepted, and you haven't already done so, sign a Contributor License Agreement (see details above).
1. Fork the desired repo, develop and test your code changes.
1. Submit a pull request.

## Development
Please go through [CSI Spec](https://github.com/container-storage-interface/spec/blob/master/spec.md) and [General CSI driver development guideline](https://kubernetes-csi.github.io/docs/developing.html) to develop a basic understanding of the CSI driver before you start.

### Requirements
* Golang 1.15.+
* [Ginkgo](https://github.com/onsi/ginkgo) in your PATH for end-to-end testing
* Docker 17.05+ for releasing

### Dependency
Dependencies are managed through go module. To build the project simply type: `make`

### Testing
* To execute all unit tests, run: `make test`
* To execute all sanity tests, run: `make test-sanity`

### Release Process
Please see [Release Process](./docs/release.md).

**Notes**:
* Sanity tests ensure that the driver complies with the CSI specification.
* E2E tests exercise various driver functionalities in a Kubernetes cluster. See [E2E Testing](./tests/e2e/README.md) for more details.

### Helm and Manifests
The Helm chart for this project is in the `charts/aws-fsx-openzfs-csi-driver` directory. 
The manifests for this project are in the `deploy/kubernetes` directory.

When updating the Helm chart:
* There is a values file in `deploy/kubernetes/values` used for generating the manifests.
* Changes should only be made to the helm template files. Do not make any changes to the manifests - these files are automatically generated from the template files.
* To generate the manifests with your changes to the helm chart, run `make generate-kustomize`.
* When adding a new resource template to the Helm chart please update the `generate-kustomize` make target, the `deploy/kubernetes/values` files, and the appropriate kustomization.yaml file(s).
