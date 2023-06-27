package driver

// TODO: Migrate other constants into this file

// Constants for default command line flag values
const (
	DefaultCSIEndpoint = "unix://tmp/csi.sock"
)

// constants for node k8s API use
const (
	// AgentNotReadyNodeTaintKey contains the key of taints to be removed on driver startup
	AgentNotReadyNodeTaintKey = "fsx.openzfs.csi.aws.com/agent-not-ready"
)
