package constants

const (
	AnnotationDomain = "profilepod.io"

	// AnnotationName is the annotation on profiler pod that specifies which PodFlame instance
	// name a specific profiler pod is associated with
	AnnotationName = AnnotationDomain + "/name"

	// AnnotationNamespace is the annotation on profiler pod that specifies which PodFlame instance
	// namespace a specific profiler pod is associated with
	AnnotationNamespace = AnnotationDomain + "/namespace"
)
