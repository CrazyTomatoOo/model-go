// Package agentgateway implements the hub-side agent gateway: the hub's single
// control-plane entrypoint for member-cluster agents (registration, heartbeat,
// task dispatch, status/log ingestion over a gRPC bidirectional stream).
// See docs/adr/0002.
package agentgateway
