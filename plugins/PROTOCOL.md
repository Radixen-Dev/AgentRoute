# Out-of-process plugin protocol (v2 — not implemented)

This directory is a placeholder for AgentRoute's v2 plugin system. **Nothing under `plugins/` is loaded,
called, or built by anything in v1** — this file documents the planned shape so the extension boundary it
will use is decided in advance, not improvised when the first real plugin shows up.

See [docs/plugins.md](../docs/plugins.md) for the (implemented, in v1) manifest-driven adapter system this
complements — manifests handle declarative wiring (write these values into that config file); this
protocol is for adapter or translator logic that genuinely needs real code, in any language.

## Why out-of-process

A `platform.Platform` or `gateway.Translator` written as a Go plugin loaded in-process would tie every
plugin to AgentRoute's exact Go toolchain version and ABI, and a panicking plugin would take the whole
binary down with it. Out-of-process plugins, communicating over gRPC via
[`github.com/hashicorp/go-plugin`](https://github.com/hashicorp/go-plugin) (the same library Terraform and
Vault use for their provider/plugin ecosystems), avoid both problems at the cost of one extra process per
loaded plugin and a serialization boundary.

## Planned shape

A plugin is a standalone executable that, on launch, speaks `go-plugin`'s handshake protocol and then
serves one or both of:

```proto
service PlatformPlugin {
  rpc Detect(DetectRequest) returns (DetectResponse);
  rpc Link(LinkRequest) returns (LinkResponse);
  rpc Unlink(UnlinkRequest) returns (UnlinkResponse);
  rpc Status(StatusRequest) returns (StatusResponse);
}

service TranslatorPlugin {
  rpc Wire(WireRequest) returns (WireResponse);
  // Streaming RPC carrying raw HTTP request/response framing, so a
  // plugin can implement arbitrary wire-protocol translation without
  // AgentRoute's core needing to understand that protocol at all.
  rpc Serve(stream TranslatorFrame) returns (stream TranslatorFrame);
}
```

These message shapes mirror `internal/platform.Platform` and `internal/gateway.Translator` field-for-field
(`DetectRequest`/`Response` ↔ `Detection`, etc.) — a plugin author should be able to read those two Go
interfaces and the generated proto side by side with no surprises.

AgentRoute's host process discovers plugins by scanning a configured directory (a `plugins/` subdirectory
of AgentRoute's own state dir, not this repo path — this `plugins/` is the *source tree* location for the
protocol definition and any first-party reference plugin, not where installed plugins are expected to
live), launches each as a subprocess, and registers whatever services it advertises into the same
`platform.Registry` / translator registry that in-tree and manifest-driven adapters use — from the core's
perspective, a plugin-backed `Platform` is indistinguishable from any other one.

## Status

Proto file, generated Go stubs, the host-side loader, and a reference plugin are all **not yet written**.
This document is the only artifact of the v2 plugin system that exists today. Treat any of the above as
subject to change once an actual plugin author's real requirements meet it.
