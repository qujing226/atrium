# Speculative Authenticated Channels

Status: research abstraction draft

Related documents:

- [FORMAL_MODEL.md](FORMAL_MODEL.md) defines the SAC properties and games more explicitly.
- [SPECIFICATION.md](SPECIFICATION.md) defines Atrium, a concrete PQ/DID instantiation of SAC.
- [EVALUATION_PLAN.md](EVALUATION_PLAN.md) defines the baseline experiments for evaluating SAC/DIG.
- [RELATED_WORK.md](RELATED_WORK.md) positions SAC against adjacent channel and systems work.

## 1. Core Idea

Speculative Authenticated Channels (SAC) are secure channels for environments where cryptographic channel establishment can complete faster than fresh authorization or identity verification. SAC separates three events that conventional authenticated channels usually collapse into one:

```text
Establish(session)    // cryptographic state exists
Decrypt(message)      // ciphertext is processed under that state
Deliver(message)      // plaintext becomes application-visible
```

The central rule is:

```text
Decrypt(m) != Deliver(m)
```

A SAC implementation may establish provisional cryptographic state and may decrypt traffic while a slow verifier is still converging. It must not treat decryption as application authorization. Plaintext obtained before verification is placed behind a Data Isolation Gate (DIG). The plaintext becomes application-visible only if verification succeeds; if verification fails, the session rolls back and isolated plaintext is destroyed.

The research claim is:

```text
Authentication freshness and cryptographic continuity can be decoupled
when application visibility is delayed behind a rollback-safe isolation gate.
```

Equivalently:

```text
SAC permits speculative cryptographic progression
without speculative semantic commitment.
```

This boundary is the core abstraction. The channel may establish keys, decrypt frames, advance ratchets, reject replays, and preserve transport continuity before the verifier converges. It may not commit the resulting plaintext to application semantics until authorization succeeds.

## 2. Motivation

Many modern systems authenticate against slow or asynchronous trust sources: decentralized identity registries, remote attestation services, replicated control planes, quorum certificates, blockchains, confidential-computing verifiers, and distributed inference schedulers. In these systems, the current authorization state may not be locally available at connection time.

A strict channel blocks until fresh verification completes. This preserves safety but turns verifier latency into user-visible or system-visible startup latency. An optimistic channel uses cached or stale evidence immediately. This hides latency but can create dirty delivery: application-visible plaintext or actions based on identity material that later proves invalid.

SAC captures the middle point. The transport may start; the cryptographic state may evolve; the application remains protected by delayed delivery semantics.

This is not equivalent to queuing ciphertext until verification completes. Ciphertext queuing preserves delivery safety, but it also blocks cryptographic progression: the receiver cannot advance ratchets, validate replay state, apply epoch transitions, or discover cryptographic failure until after authorization returns. SAC instead permits those protocol-level actions during the verification window while preventing application-visible commitment.

## 3. System Model

A SAC system consists of:

| Component | Role |
| --- | --- |
| Endpoint | Runs the channel protocol and owns local cryptographic state. |
| Peer identity | The claimed identity or authorization principal of the remote endpoint. |
| Trust verifier | A slow or asynchronous source of fresh identity or authorization truth. |
| Channel state | Cryptographic session state used to process frames or messages. |
| DIG | Isolation boundary between decrypted plaintext and application-visible effects. |
| Application | Consumer of delivered plaintext and producer of outbound plaintext. |

The verifier may represent a DID registry, attestation service, consensus-backed identity state, certificate transparency proof, key transparency log, scheduler authorization service, or any other delayed trust source. SAC does not require a specific verifier construction.

## 4. State Model

SAC uses four abstract states:

| State | Meaning |
| --- | --- |
| `IDLE` | No usable channel state exists. |
| `SPECULATIVE` | Cryptographic state exists, but fresh authorization is pending. |
| `VERIFIED` | Fresh authorization has validated the session. |
| `ABORTED` | The session is invalid and cannot deliver plaintext. |

The core transitions are:

| Current State | Event | Condition | Action | Next State |
| --- | --- | --- | --- | --- |
| `IDLE` | establish | fresh authorization already available | initialize channel state | `VERIFIED` |
| `IDLE` | establish | cached or provisional evidence only | initialize provisional channel state; start verifier | `SPECULATIVE` |
| `SPECULATIVE` | decrypt | ciphertext valid | advance channel state; isolate plaintext | `SPECULATIVE` |
| `SPECULATIVE` | verify_success | verifier accepts evidence | atomically release DIG buffer | `VERIFIED` |
| `SPECULATIVE` | verify_failure | verifier rejects evidence | destroy isolated plaintext and session state | `ABORTED` |
| `SPECULATIVE` | timeout or resource_exhaustion | local bound exceeded | destroy isolated plaintext and session state | `ABORTED` |
| `VERIFIED` | decrypt | ciphertext valid | deliver plaintext | `VERIFIED` |
| `VERIFIED` | fatal_error | unrecoverable protocol error | destroy session state | `ABORTED` |
| `ABORTED` | any | any | discard | `ABORTED` |

The state machine is local. A peer cannot make a receiver `VERIFIED` by claiming success. The receiver's own verifier controls the transition.

## 5. Data Isolation Gate

DIG is the semantic boundary that makes speculation safe. It is not merely an implementation queue. It defines when plaintext acquires application meaning.

In `SPECULATIVE`, the protocol may compute:

```text
p = Decrypt(c)
```

but must perform:

```text
Buffer(session, p)
```

not:

```text
Deliver(session, p)
```

The release rule is:

```text
Deliver(session, p) => State(session) = VERIFIED
```

The rollback rule is:

```text
Abort(session) => forall p in DIG(session), not Deliver(session, p)
```

The non-delivery rule is:

```text
State(session) = SPECULATIVE => forall p, not ApplicationVisible(p)
```

Here `ApplicationVisible` means visible through the protocol's application delivery interface. It does not claim absence of timing, memory, traffic, backpressure, or scheduling observability. SAC prevents unauthorized application-visible semantic effects; it does not provide a physical non-interference guarantee.

## 6. Formal Properties

### 6.1 Authorized Delivery

Let `Authorized(s)` mean the verifier has accepted the identity or authorization evidence for session `s`. Let `Deliver(s, m)` mean plaintext `m` is released to the application for session `s`.

SAC requires:

```text
Deliver(s, m) => Authorized(s)
```

This is the primary safety property. It permits provisional channel state before authorization, but it forbids application-visible plaintext before authorization.

### 6.2 Speculative Non-Delivery

Let `Speculative(s)` mean session `s` is in `SPECULATIVE`.

```text
Speculative(s) => not exists m. Deliver(s, m)
```

Messages may be decrypted and buffered, but not delivered.

### 6.3 Rollback Safety

Let `Buffered(s, m)` mean plaintext `m` is in the DIG buffer for session `s`.

```text
Abort(s) and Buffered(s, m) => always not Deliver(s, m)
```

An aborted speculative session has no partial delivery semantics. Rollback is all-or-nothing with respect to application visibility.

### 6.4 Eventual Delivery Under Valid Evidence

If the verifier eventually accepts a speculative session and the implementation does not exceed local resource bounds, buffered messages are delivered in protocol order:

```text
Speculative(s) and VerifySuccess(s)
=> eventually forall m in DIG(s). Deliver(s, m) in order
```

This is a liveness property conditioned on verifier response and local resource availability.

### 6.5 State Continuity

SAC allows cryptographic state to advance while delivery is blocked:

```text
Speculative(s) and Decrypt(c_i) => StateAdvance(s)
```

This property distinguishes SAC from designs that simply queue ciphertext until authorization completes. The channel can preserve ratchet continuity, replay checks, and transport progress during the speculative window.

### 6.6 Non-Equivalence to Queues and Callbacks

SAC is not a ciphertext queue, because valid ciphertext may be decrypted and may advance channel state before authorization completes. SAC is not only a delayed callback API, because the protocol state machine defines which speculative actions are permitted, which actions are forbidden, and how rollback destroys isolated plaintext. SAC is not an async-auth wrapper, because verification status controls the channel's delivery state rather than merely annotating an already delivered stream.

The distinction can be summarized as:

```text
wait-then-decrypt:    no speculative crypto progress, no dirty delivery
decrypt-then-deliver: speculative crypto progress, possible dirty delivery
SAC:                 speculative crypto progress, no semantic commitment
```

The third line is the contribution: SAC hides verifier latency without letting provisional authorization cross the application boundary.

## 7. Adversary Model

The adversary may control the network, delay verifier responses, replay messages, reorder messages, cause cache staleness, trigger verifier races, and attempt to exhaust isolation resources. The adversary may also observe timing, message sizes, aborts, and buffering pressure unless a concrete instantiation adds separate metadata protection.

The adversary wins the SAC delivery-safety game if an honest endpoint executes `Deliver(s, m)` for a session `s` where `Authorized(s)` is false.

SAC assumes:

- the verifier has a defined safety property;
- the cryptographic channel prevents plaintext forgery under uncompromised session keys;
- the implementation enforces the DIG state machine;
- local resource bounds are finite and enforced.

## 8. Proof Sketch

The delivery-safety argument is state-machine based.

In `IDLE`, no plaintext can be delivered because no session exists. In `SPECULATIVE`, the only permitted action after decryption is insertion into DIG, and the delivery function is disabled. In `ABORTED`, DIG is cleared and delivery remains disabled. Therefore, the only state where `Deliver(s, m)` can occur is `VERIFIED`.

The transition to `VERIFIED` requires verifier acceptance. Thus:

```text
Deliver(s, m)
=> State(s) = VERIFIED
=> Authorized(s)
```

An adversary can violate this property only by:

- causing the verifier to accept false evidence;
- forging cryptographic evidence required by the concrete instantiation;
- exploiting an implementation bug that bypasses DIG.

Therefore, for an adversary `A`:

```text
Pr[A wins delivery safety]
<= Pr[verifier safety failure]
 + Adv[crypto forgery]
 + Pr[DIG implementation failure]
```

This proof deliberately avoids claiming non-observability. Speculative computation may be visible through timing or resource behavior. The property is about application-authorized semantic delivery.

## 9. Instantiation Requirements

A concrete SAC instantiation must define:

- the verifier and its safety property;
- the cryptographic channel establishment mechanism;
- the exact session identifier;
- message ordering and replay rules;
- DIG resource bounds;
- verifier timeout behavior;
- abort propagation;
- the application delivery boundary;
- test cases showing that invalid speculative sessions do not deliver plaintext.

## 10. Atrium as an Instantiation

Atrium instantiates SAC for decentralized post-quantum identity channels. It uses DID-anchored identity material, ML-KEM for key establishment, ML-DSA for control authentication, AEAD-protected data frames, an epoch ratchet for long sessions, and a fixed binary frame format for interoperability.

Those choices are engineering decisions for one deployment context. The SAC abstraction does not depend on DID, ML-KEM, ML-DSA, route IDs, or Atrium's frame layout. A different SAC instantiation could use remote attestation, a cluster scheduler, certificate transparency, a key-transparency log, or confidential-computing evidence as its verifier.
