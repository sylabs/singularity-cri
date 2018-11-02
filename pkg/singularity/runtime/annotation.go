package runtime

const (
	// AnnotationContainerType is used to notify runtime what type of process
	// should be run (pod or container).
	AnnotationContainerType = "io.sylabs.oci.runtime.type"
	// AnnotationSyncSocket is used to pass path to a  sync socket to the runtime.
	AnnotationSyncSocket = "io.sylabs.oci.runtime.cri-sync-socket"

	// ContainerTypePod denotes that pod should be run.
	ContainerTypePod = "pod"
)
