// Package clusteragent implements the member-cluster agent: registration,
// credential rotation, heartbeat, accelerator-capacity reporting, task pull
// and status/log push over a gRPC bidirectional stream to the hub's
// agent-gateway. Pure carrier — no reconcile logic. See docs/adr/0002.
package clusteragent
