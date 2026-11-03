// Package controllermanager implements the hub-side orchestration layer: it
// translates user requests into (target cluster, task spec) records in
// Postgres. It holds no workload CRDs — those live in member clusters.
// See docs/adr/0002.
package controllermanager
