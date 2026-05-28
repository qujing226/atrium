# Evaluation Plan

Status: research evaluation draft

Related documents:

- [ABSTRACTION.md](ABSTRACTION.md) defines the SAC abstraction.
- [FORMAL_MODEL.md](FORMAL_MODEL.md) states the delivery-safety model.
- [SPECIFICATION.md](SPECIFICATION.md) defines Atrium as a concrete instantiation.
- [RELATED_WORK.md](RELATED_WORK.md) defines the comparison surface.

## 1. Purpose

This document defines the experiments needed to evaluate Speculative Authenticated Channels (SAC) and the Atrium instantiation. The evaluation should answer one central question:

```text
Can a channel hide verifier latency without allowing invalid application-visible delivery?
```

The evaluation is not primarily about proving that ML-KEM or ML-DSA are fast. Those are implementation choices. The core evaluation target is the tradeoff between startup latency, delivery safety, rollback behavior, and isolation overhead under delayed authorization.

## 2. Research Questions

### RQ1. Latency Hiding

Does speculative cryptographic establishment keep channel startup latency flat as verifier latency increases?

Primary metrics:

- Time-to-First-Frame (TTFF): application request to first protected frame written.
- Time-to-First-Verified-Delivery (TTFVD): application request to first delivered plaintext.
- Verification convergence latency.

### RQ2. Dirty Delivery Prevention

Does DIG prevent invalid application-visible plaintext when cached identity or authorization evidence is stale?

Primary metrics:

- invalid session attempts;
- invalid application deliveries;
- abort rate;
- rollback latency;
- buffered bytes discarded.

### RQ3. Cost of Isolation

What memory, CPU, and latency overhead does DIG introduce compared with optimistic immediate delivery?

Primary metrics:

- peak DIG buffer bytes;
- buffered message count;
- memory overhead per speculative session;
- flush latency on verification success;
- abort cleanup latency.

### RQ4. Long-Session Refresh

How often does Epoch-KEM trigger under different entropy budgets, and what is the bandwidth and latency cost?

Primary metrics:

- Epoch-KEM count per session;
- bytes sent per epoch refresh;
- data-plane interruption during refresh;
- post-refresh message latency.

## 3. Baselines

The minimum baselines are:

| Name | Behavior | Expected Strength | Expected Weakness |
| --- | --- | --- | --- |
| Strict Sync Verification | Blocks channel establishment until verifier returns. | No dirty delivery. | Startup latency grows with verifier latency. |
| Optimistic Immediate Delivery | Uses cached evidence and delivers plaintext immediately. | Low startup latency. | Dirty delivery under stale evidence. |
| SAC/DIG | Uses cached evidence but isolates plaintext until verification. | Low protected-frame latency and no dirty delivery. | Memory overhead and delayed application delivery. |

Optional baselines:

- TLS-like local trust anchor baseline for local-verification lower bound.
- Ciphertext queue baseline that does not advance ratchets until verification.
- Atrium without Epoch-KEM for long-session overhead comparison.

## 4. Workloads

The evaluation should include at least three workloads:

### 4.1 Interactive Small Messages

Small messages model chat, control-plane RPC, and agent coordination.

```text
payload size: 64 B, 256 B, 1 KiB
message count: 1, 10, 100 per session
```

### 4.2 Burst During Verification Window

Burst traffic stresses DIG memory and release behavior.

```text
payload size: 1 KiB, 16 KiB
burst size: 32, 64, 128 messages
verifier delay: 100 ms to 30 s
```

### 4.3 Long Session

Long sessions exercise ratchet and Epoch-KEM scheduling.

```text
duration: 1 min, 10 min, 1 hour simulated
message rate: 1/s, 10/s, 100/s
entropy budget: low, medium, high
```

## 5. Verifier and Attack Scenarios

The verifier should be modeled with controllable delay and controllable truth changes.

### 5.1 Verifier Delay

Use deterministic and distributional delays:

```text
0 ms, 10 ms, 100 ms, 500 ms, 1 s, 5 s, 15 s, 30 s
```

Also evaluate log-normal or long-tail distributions if modeling real decentralized systems.

### 5.2 Stale Evidence

Inject stale cached identity material:

```text
stale rate: 0%, 1%, 5%, 10%, 25%, 50%
```

A stale session is invalid if the verifier would reject the cached material used to establish it.

### 5.3 Key Rotation Shock

Simulate a sudden key rotation event that invalidates a large fraction of caches.

```text
rotation time: t0
affected identities: 10%, 50%, 100%
cache refresh policy: immediate, TTL, stale-while-revalidate
```

### 5.4 Proof Starvation

Delay verifier responses beyond the speculative timeout to test resource cleanup.

Expected result:

```text
SAC/DIG aborts and releases no plaintext.
Optimistic immediate delivery may already have delivered invalid plaintext.
Strict sync waits or times out before protected traffic.
```

## 6. Metrics

### 6.1 Latency Metrics

- `ttff_ms`: application request to first protected frame written.
- `ttfvd_ms`: application request to first verified delivery.
- `verify_ms`: session start to verifier result.
- `rollback_ms`: verifier failure to session abort completion.
- `flush_ms`: verifier success to completion of DIG flush.

### 6.2 Safety Metrics

- `invalid_attempts`: sessions established from stale or invalid evidence.
- `invalid_deliveries`: invalid plaintext delivered to the application.
- `dirty_delivery_rate = invalid_deliveries / invalid_attempts`.
- `aborted_invalid_sessions`.
- `buffered_invalid_messages_discarded`.

### 6.3 Resource Metrics

- `peak_dig_bytes`.
- `peak_dig_messages`.
- `alloc_bytes_per_session`.
- `cpu_ms_handshake`.
- `cpu_ms_verify_callback`.
- `bytes_per_session`.
- `bytes_per_epoch_refresh`.

### 6.4 Ratchet Metrics

- `epoch_kem_count`.
- `messages_per_epoch`.
- `seconds_per_epoch`.
- `post_refresh_latency_ms`.
- `refresh_bytes`.

## 7. Experimental Methodology

Each experiment should report:

- sample count;
- median;
- p95;
- p99;
- mean only as secondary information;
- confidence intervals or bootstrap error bars;
- random seed for reproducibility;
- machine type and network topology;
- verifier delay distribution;
- cache policy;
- DIG bounds.

Experiments comparing multiple protocols MUST use the same workload, verifier delay trace, key-rotation schedule, and network topology.

## 8. Expected Results

The expected qualitative results are:

| Scenario | Strict Sync | Optimistic Immediate | SAC/DIG |
| --- | --- | --- | --- |
| Increasing verifier latency | TTFF grows with verifier latency. | TTFF stays low. | TTFF stays low. |
| Stale cached evidence | No dirty delivery; startup blocks. | Dirty delivery rate tracks stale rate. | Invalid sessions abort without dirty delivery. |
| Burst before verification | No burst accepted before verification. | Delivered immediately, including invalid data. | Buffered, then flushed or discarded. |
| Proof starvation | Blocks or times out. | May deliver before proof arrives. | Aborts at speculative timeout. |

The key result is not that SAC always minimizes application-visible delivery latency. SAC may delay delivery until verification. The key result is that it preserves transport progress and cryptographic continuity while preventing invalid application-visible semantic effects.

## 9. Minimum Reproducible Artifact

A minimum artifact should include:

- a deterministic verifier with configurable delay and stale-evidence injection;
- implementations of Strict Sync, Optimistic Immediate Delivery, and SAC/DIG;
- scripts to run latency, dirty-delivery, DIG memory, and Epoch-KEM experiments;
- CSV output with the metrics above;
- plotting scripts for the main figures;
- test cases proving invalid speculative sessions do not call application delivery.

## 10. Paper Figures

Recommended first figures:

1. TTFF vs verifier latency.
2. TTFVD vs verifier latency.
3. Dirty delivery rate vs stale evidence rate.
4. Peak DIG memory vs verifier delay and burst size.
5. Epoch-KEM bandwidth overhead vs entropy budget.

The figures should make the core abstraction visible: strict verification sacrifices latency, optimistic delivery sacrifices safety, and SAC/DIG separates protected transport progress from authorized application delivery.
