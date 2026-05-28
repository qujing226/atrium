# Paper Outline: Speculative Authenticated Channels

Working title:

```text
Speculative Authenticated Channels:
Delivery-Safe Cryptographic Progress Under Delayed Authorization
```

## 1. Thesis

The paper should argue one focused claim:

```text
Authenticated channel semantics can permit speculative cryptographic
progression without speculative semantic commitment.
```

The paper should not claim to introduce a universal secure transport, a replacement for TLS or Signal, or a fundamentally new post-quantum primitive. Post-quantum DID transport is the concrete instantiation; SAC/DIG is the research contribution.

## 2. Abstract Shape

Modern systems increasingly authenticate against slow or asynchronous trust sources, such as decentralized identity registries, remote attestation services, key-transparency systems, quorum-backed authorization, and distributed runtime schedulers. Existing secure channels usually treat cryptographic establishment, peer authentication, and application delivery as a single convergence point. This makes strict designs safe but latency-sensitive, while naive optimistic designs hide latency but risk dirty delivery under stale authorization.

SAC separates cryptographic progress from semantic commitment. A SAC endpoint may establish provisional channel state, decrypt valid ciphertext, advance ratchets, and maintain replay state while fresh authorization is pending. A Data Isolation Gate prevents the resulting plaintext from becoming application-visible until verification succeeds; failed verification aborts the session and destroys isolated plaintext. Atrium instantiates SAC with DID-anchored identity and post-quantum cryptography.

## 3. Contributions

1. Define SAC, an authenticated channel abstraction for delayed authorization environments.
2. Formalize DIG as a delivery-safety primitive with `Decrypt(m) != Deliver(m)`.
3. Show why SAC differs from ciphertext queues, delayed callbacks, and naive optimistic delivery.
4. Present Atrium as a minimal PQ/DID instantiation of SAC.
5. Evaluate strict verification, optimistic immediate delivery, and SAC/DIG under identical verifier-delay and stale-evidence workloads.

## 4. Introduction

The introduction should lead with the delayed-authorization problem, not with post-quantum cryptography. The motivating examples are DID registries, remote attestation, key transparency, confidential runtime admission, and distributed inference schedulers. PQ appears later as a practical instantiation choice.

The key contrast:

```text
Strict:     verify -> decrypt -> deliver
Optimistic: decrypt -> deliver -> maybe rollback too late
Queue:      verify -> decrypt queued ciphertext -> deliver
SAC:        decrypt/advance -> verify -> deliver or abort
```

The punchline is that only SAC preserves cryptographic state continuity while preventing unauthorized application-visible semantic effects.

## 5. Model

Define:

- `Establish(session)`
- `Decrypt(session, ciphertext)`
- `AdvanceState(session)`
- `Buffer(session, plaintext)`
- `Deliver(session, plaintext)`
- `Commit(session, plaintext)`
- `Authorized(session)`

State:

```text
IDLE -> SPECULATIVE -> VERIFIED
                    -> ABORTED
```

Core properties:

- Authorized delivery: `Deliver(s, m) => Authorized(s)`
- Speculative non-delivery: `SPECULATIVE(s) => not exists m. Deliver(s, m)`
- Semantic non-commitment: `SPECULATIVE(s) and Decrypt(s, c)->m => not Commit(s, m)`
- Rollback safety: aborted buffered plaintext is never delivered
- State continuity: valid speculative ciphertext may advance cryptographic state

## 6. Design

Describe DIG as the mechanism that enforces the model. Keep this section independent of Atrium wire format. The essential design elements are:

- application delivery boundary;
- isolation buffer;
- ordered release;
- all-or-nothing discard on abort;
- finite speculative memory and time bounds;
- verifier-controlled transition to `VERIFIED`;
- explicit side-channel stance.

The side-channel stance must be narrow: SAC prevents unauthorized application-visible semantic effects; it does not claim physical non-interference or absence of timing/resource leakage.

## 7. Atrium Instantiation

Atrium should be presented as one implementation of the abstraction:

- DID identity material as authorization evidence;
- delayed DID/trust-anchor verification as the slow verifier;
- ML-KEM for establishment and epoch refresh;
- ML-DSA for control-frame authentication;
- AEAD-protected data frames;
- bidirectional ratchet and Epoch-KEM for long sessions;
- fixed header and route ID as engineering choices.

This section should stay short. The detailed wire format belongs in [SPECIFICATION.md](SPECIFICATION.md), not in the paper body.

## 8. Evaluation

Primary comparison:

1. Strict synchronous verification.
2. Optimistic immediate delivery.
3. SAC/DIG.

Main metrics:

- time to first protected frame;
- time to first verified delivery;
- invalid application deliveries;
- rollback rate and rollback latency;
- DIG memory occupancy;
- verifier convergence latency;
- epoch refresh overhead.

The central expected result is not "zero leakage." It is: SAC can preserve early cryptographic progress while maintaining zero invalid application deliveries in the tested stale-evidence workloads.

## 9. Related Work

Use [RELATED_WORK.md](RELATED_WORK.md) as the source. The paper should position against:

- TLS 1.3 and secure channel abstractions;
- Signal, Double Ratchet, and MLS;
- constructive/UC-style channel definitions;
- optimistic concurrency control and transactional memory;
- speculative execution side-channel work;
- delayed trust systems such as DID, attestation, and key transparency.

## 10. Limitations

State these limitations explicitly:

- no metadata privacy claim;
- no physical side-channel non-interference claim;
- no verifier-liveness guarantee;
- no proof of a concrete DID method or consensus layer;
- no protection after endpoint compromise and application delivery;
- DIG correctness depends on implementation discipline.

## 11. One-Sentence Contribution

```text
SAC is a delivery-safe authenticated channel abstraction that lets
cryptographic state advance under delayed authorization while preventing
unauthorized plaintext from becoming application-visible.
```
