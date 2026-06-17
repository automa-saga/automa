# automa Specifications

> Status: **proposed** — spec-first effort, under review.

automa is intended to exist in more than one language (Go first, then others such
as Kotlin, Python, and Rust) for the same audience and use cases described in
[docs/README.md](../README.md).

For a multi-language project the **product is the specification**, not any single
implementation. Two runtimes that both "implement the Saga pattern" are worthless
to a user if a workflow — especially a *durable* one — behaves differently
depending on the language it happens to be written in. The specifications in this
directory define the behavior that every conformant implementation MUST exhibit,
so that:

- a journal written by one implementation can be understood (and, where the
  topology allows, reasoned about) by another, and
- the resume/recovery semantics are identical everywhere, and
- the obligations placed on workflow authors (e.g. idempotency) are one promise,
  not a per-language footnote.

## Principles

1. **Spec first, implementation second.** Observable behavior is defined here in
   language-neutral terms. The Go implementation is the **first conformant
   implementation**, not the definition. Where the Go code and a spec disagree,
   the spec is authoritative and the code is a bug.
2. **On-disk formats are contracts.** Any artifact written to disk or exchanged
   between processes (the durability journal, the report tree) has a versioned,
   language-neutral schema. Field names and value enumerations are fixed by the
   spec, not by a language's default serializer.
3. **Conformance is testable.** Each spec is accompanied by language-neutral
   **conformance fixtures** (input artifacts + expected outcomes, expressed as
   plain JSON). Every implementation MUST pass the shared fixtures. The fixtures
   are the source of truth for cross-language agreement.
4. **The core stays small.** The smaller the specified core, the cheaper it is to
   keep N implementations in agreement. Features outside the core (durable
   timers, distributed execution, signals) are explicitly out of scope until and
   unless they are specified here.

## Conformance keywords

The key words **MUST**, **MUST NOT**, **REQUIRED**, **SHALL**, **SHOULD**,
**SHOULD NOT**, **MAY**, and **OPTIONAL** in these documents are to be
interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119).

## Specifications

| Spec | Status | Description |
|------|--------|-------------|
| [Core](core-spec.md) | proposed | The saga model: step lifecycle, workflow execution loop, execution/rollback modes, composability, state model, and report tree. The foundation all implementations MUST satisfy. |
| [Durability](durability-spec.md) | proposed | **Extends Core.** Journal format, execution state machine, persistence ordering, and resume/recovery semantics for crash-recoverable sequential sagas. |

Specs layer: **Core** is the foundation; **Durability** is an extension built on
top of it. Future features are additional extensions, each with its own spec and
conformance fixtures, so the core stays small and stable.

### Extension roadmap and ordering

Extensions are added **only after core v1 and durability v1 are frozen** and the
Go reference implementation passes their conformance fixtures. Planned order, by
increasing risk to the core model:

1. **Timeout** — per-step / per-workflow deadline. Orthogonal: a timed-out step
   is simply a failure and flows through the existing mode/rollback machinery.
2. **Retry / backoff** — execution-layer extension. Its main interaction is the
   idempotency contract already defined by durability (a retried step has the
   same obligation as a resumed one).
3. **Branching / looping** — **deferred until there is concrete demand.** These
   make topology conditional/dynamic, which conflicts with durability's core
   constraint that topology is a static, reconstructible, ordered list. They MUST
   NOT be added without a topology-model revision that durability can survive
   (e.g. journaling the branch taken / iteration index). The target use cases
   (CLI, provisioning, migration) are overwhelmingly linear, so this is low
   priority.

Parallel/concurrent step execution is likewise a post-v1 extension, not a core
v1 concern.

## Relationship to design docs

`docs/durability.md` is the **design rationale** (why the feature exists, what it
offers, the tradeoffs). This directory holds the **normative specification** (what
an implementation MUST do). When they differ, the spec governs behavior; the
design doc governs intent.
