# Atrium Prototype Deployment Plan

Status: evaluation deployment draft

Related documents:

- [ABSTRACTION.md](ABSTRACTION.md) defines Speculative Authenticated Channels (SAC).
- [EVALUATION_PLAN.md](EVALUATION_PLAN.md) defines the research questions and baselines.
- [SPECIFICATION.md](SPECIFICATION.md) defines the Atrium wire protocol.

## 1. Goal

This plan describes a minimal distributed deployment for evaluating Atrium as a SAC instantiation. The goal is not to prove that Atrium is faster than TLS in ordinary Web PKI settings. The goal is to measure whether SAC can preserve early cryptographic progress while preventing invalid application-visible delivery when authorization evidence is delayed or stale.

The deployment should support industrial baselines and mechanism ablations. The primary baselines are:

1. TLS 1.3 local authentication.
2. TLS 1.3 0-RTT, if supported by the selected TLS implementation.
3. TLS 1.3 post-handshake authentication, if supported by the selected TLS implementation.
4. TLS 1.3 plus an application-layer external verifier.
5. SAC/DIG over the same workload and verifier schedule.

The mechanism ablations are strict synchronous verification, optimistic immediate delivery, ciphertext queue, and SAC/DIG. These ablations explain causality, but the standard TLS mechanisms and the TLS+external-verifier construction provide the credible systems comparison.

## 2. Minimal Topology

The first deployment should use four independently configurable processes. They may run on one machine for functional tests and on separate hosts for WAN experiments.

| Node | Role | Requirement |
| --- | --- | --- |
| Initiator | Starts sessions and sends application payloads. | Records send time, first protected frame time, first verified delivery time, and aborts. |
| Responder | Accepts sessions and receives payloads. | Enforces the selected delivery policy and records delivery/rollback events. |
| Relay | Routes frames by route ID. | Does not inspect payloads and can inject delay/drop/reorder policies. |
| Verifier | Returns fresh authorization status. | Supports valid evidence, stale evidence, delayed success, delayed failure, and timeout scenarios. |

The verifier is intentionally abstract. In early experiments it can be a deterministic mock service with configurable latency and decision schedules. A real DID or blockchain-backed verifier is useful later, but it is not required to test the SAC property.

## 3. Deployment Phases

### 3.1 Local Functional Deployment

Run all four processes on localhost. This phase verifies message flow, state transitions, DIG behavior, and telemetry without WAN noise.

Required scenarios:

- TLS 1.3 local authentication completes and delivers normally;
- TLS 1.3 0-RTT is either implemented with a library that exposes early data or explicitly reported as unsupported;
- TLS 1.3 post-handshake authentication is either implemented with a library that exposes it or explicitly reported as unsupported;
- TLS 1.3 + app-layer external verifier withholds delivery until verifier success and discards on verifier failure;
- fresh evidence succeeds before any data arrives;
- cached evidence enters `SPECULATIVE`, then verifies;
- cached evidence enters `SPECULATIVE`, then fails;
- verifier timeout;
- DIG byte limit exceeded;
- duplicate or stale sequence rejected;
- stale epoch rejected.

### 3.2 Controlled Delay Deployment

Run the same topology with artificial verifier latency. Network RTT may still be local; only verifier delay changes. This isolates the core SAC question from WAN variability.

Required verifier delay distribution:

```text
0 ms, 10 ms, 100 ms, 500 ms, 1 s, 5 s, 15 s
```

For each delay, run all TLS variants and SAC/DIG against the same payload sizes, session counts, cache state, and failure schedule.

### 3.3 WAN Deployment

Place nodes in separate regions only after local and controlled-delay experiments are stable. WAN deployment should measure interaction between transport RTT and verifier latency, not replace the controlled experiment.

Recommended placement:

| Node | Example region | Purpose |
| --- | --- | --- |
| Initiator | Asia | Client-side startup latency. |
| Responder | Europe | Cross-region responder path. |
| Relay | North America | Untrusted routing hop. |
| Verifier | Separate region | Independent delayed authorization source. |

## 4. Telemetry

Each process should emit structured logs with monotonic timestamps. Logs must not contain plaintext, chain keys, KEM shared secrets, private keys, or speculative application interpretation.

Required metrics:

| Metric | Meaning |
| --- | --- |
| `time_to_first_frame` | Application send request to first protected frame write. |
| `time_to_first_verified_delivery` | Application send request to first plaintext delivery after verification. |
| `verification_latency` | Verifier request start to verifier decision. |
| `invalid_delivery_count` | Number of application-visible deliveries for sessions later rejected by verifier. |
| `dig_buffer_bytes_peak` | Peak isolated plaintext bytes per session. |
| `dig_buffer_messages_peak` | Peak isolated plaintext message count per session. |
| `rollback_latency` | Verification failure to session abort and buffer clear. |
| `abort_reason` | Verification failure, timeout, replay, epoch mismatch, DIG overflow, or parse/auth failure. |
| `epoch_kem_count` | Number of epoch refreshes per session. |
| `mode` | One of `tls13_local_auth`, `tls13_0rtt`, `tls13_post_handshake_auth`, `tls13_app_external_verifier`, `sac`, or a mechanism ablation. |

## 5. Workloads

Use small, reproducible workloads before adding application realism:

- handshake only;
- 1 message of 1 KiB;
- 64 messages of 1 KiB;
- 1 MiB burst split into fixed-size frames;
- stale-evidence workload with deterministic verification failure;
- key-rotation shock where a fixed percentage of cached evidence becomes invalid.

The main stale-evidence experiment should report both attempted invalid sessions and actual invalid application deliveries. Reporting only successful handshakes is insufficient.

## 6. Expected Comparisons

The expected qualitative behavior is:

| Mode | Early cryptographic progress | Invalid delivery risk | Verifier latency exposure |
| --- | --- | --- | --- |
| TLS 1.3 local auth | yes | not modeled for external stale evidence | none unless external verifier is added |
| TLS 1.3 0-RTT | yes, for early data after resumption | replay and freshness constraints; not an external verifier solution | low for early data, but not tied to external verifier freshness |
| TLS 1.3 post-handshake authentication | yes | depends on application policy after deferred auth | does not define external verifier delivery semantics by itself |
| TLS 1.3 + app-layer external verifier | yes | no if application gate is correct | low for transport, high for verified delivery |
| SAC/DIG | yes | no if DIG is correct | low for cryptographic progress, high for verified delivery |
| Ciphertext queue ablation | no speculative decrypt/state advance | no | high for cryptographic progress |

The paper should not claim that SAC beats TLS 1.3. TLS 1.3 local authentication is a mature lower-bound reference when authorization is locally available. TLS 1.3 0-RTT and post-handshake authentication are standard mechanisms that must be discussed or measured when implementation support exists. The relevant delayed-external-authorization comparison is TLS+app-layer external verifier versus SAC/DIG.

## 7. Limitations

This deployment does not evaluate metadata privacy, physical side channels, production DID method security, consensus liveness, or endpoint compromise after delivery. It evaluates the SAC/DIG delivery boundary under controlled verifier delay and stale-evidence workloads.
