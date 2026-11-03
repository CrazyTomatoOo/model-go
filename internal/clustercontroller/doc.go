// Package clustercontroller implements the member-cluster operator: it
// reconciles the platform's TrainingJob/InferenceService CRDs locally and
// translates them into execution-plane resources (Volcano Jobs, Deployments,
// LWS). See docs/adr/0002 and docs/adr/0004.
package clustercontroller
