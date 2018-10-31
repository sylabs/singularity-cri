package kube

const (
	// AnnotationType is used to notify runtime that it should run a pod.
	AnnotationType = "io.sylabs.oci.runtime.type"
	// AnnotationSyncSocket is used to pass path to a  sync socket to the runtime.
	AnnotationSyncSocket = "io.sylabs.oci.runtime.cri-sync-socket"
)
