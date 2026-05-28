# Formal Model for Speculative Authenticated Channels

Status: research model draft

Related documents:

- [ABSTRACTION.md](ABSTRACTION.md) defines the SAC abstraction.
- [SPECIFICATION.md](SPECIFICATION.md) defines Atrium as a concrete instantiation.
- [EVALUATION_PLAN.md](EVALUATION_PLAN.md) defines the baseline experiments.
- [RELATED_WORK.md](RELATED_WORK.md) positions the model against adjacent work.

## 1. Purpose

This document formalizes the core security properties of Speculative Authenticated Channels (SAC). It is intentionally independent of Atrium's concrete DID, post-quantum, routing, and wire-format choices. Atrium is one instantiation of this model.

The central distinction is:

```text
Decrypt(m) != Deliver(m)
```

SAC permits cryptographic state to advance before fresh authorization converges. Its safety claim is that application-visible semantic delivery is delayed until authorization succeeds.

The abstraction is therefore:

```text
speculative cryptographic progression
without speculative semantic commitment
```

## 2. Entities and State

Let `P` be a set of protocol participants. A local protocol instance is:

```text
Pi(i, s)
```

where `i in P` is the local participant and `s` is a local session identifier.

Each instance maintains:

```text
peer_id       claimed peer identity or authorization principal
sid           session identifier
st            delivery state in {IDLE, SPECULATIVE, VERIFIED, ABORTED}
ck_send       local send channel state
ck_recv       local receive channel state
B_iso         isolation buffer
V             verifier handle or pending verifier query
```

The verifier is modeled as an oracle:

```text
Verify(peer_id, evidence) -> {accept, reject, pending}
```

The verifier represents any delayed trust source: DID registry, remote attestation service, key-transparency log, consensus proof, scheduler authorization service, or equivalent mechanism.

## 3. Events

The model uses the following abstract events:

```text
Establish(s, evidence)
Recv(s, c)
Decrypt(s, c) -> m
Buffer(s, m)
Deliver(s, m)
VerifySuccess(s)
VerifyFailure(s)
Abort(s)
RevealState(s, t)
Corrupt(i)
```

`Recv` is network input. `Decrypt` is cryptographic processing. `Deliver` is the first application-visible semantic effect. `Buffer` stores plaintext behind DIG without making it application-visible.

## 4. State Transition System

The delivery state transition function is:

```text
delta: State x Event -> State x Action
```

The required transitions are:

```text
delta(IDLE, Establish with fresh authorization)
  -> VERIFIED, InitChannel

delta(IDLE, Establish with provisional evidence)
  -> SPECULATIVE, InitChannel + StartVerify

delta(SPECULATIVE, Recv valid ciphertext)
  -> SPECULATIVE, Decrypt + AdvanceState + Buffer

delta(SPECULATIVE, VerifySuccess)
  -> VERIFIED, ReleaseBufferInOrder

delta(SPECULATIVE, VerifyFailure)
  -> ABORTED, ClearBuffer + DestroyState

delta(SPECULATIVE, Timeout or ResourceBoundExceeded)
  -> ABORTED, ClearBuffer + DestroyState

delta(VERIFIED, Recv valid ciphertext)
  -> VERIFIED, Decrypt + AdvanceState + Deliver

delta(VERIFIED, FatalError)
  -> ABORTED, DestroyState

delta(ABORTED, any event)
  -> ABORTED, Discard
```

No transition from `SPECULATIVE` may execute `Deliver`.

## 5. Security Predicates

### 5.1 Authorization

`Authorized(s)` means the verifier has accepted the identity or authorization evidence for session `s`.

```text
Authorized(s) := Verify(peer_id_s, evidence_s) = accept
```

The exact evidence structure is instantiation-specific.

### 5.2 Application Visibility

`ApplicationVisible(s, m)` means message `m` has crossed the protocol delivery boundary for session `s`. Examples include application callbacks, command execution, application queue insertion, user notifications, durable application writes, and semantic acknowledgments.

The model does not treat timing, memory pressure, packet counts, backpressure, or abort observability as forbidden visibility. Those are side channels and operational signals. SAC's claim is restricted to application-visible semantic effects.

### 5.3 Isolation

`Buffered(s, m)` means plaintext `m` is held inside DIG for session `s`.

```text
Buffered(s, m) => not ApplicationVisible(s, m)
```

## 6. Core Properties

### 6.1 Authorized Delivery

```text
forall s, m:
  Deliver(s, m) => Authorized(s)
```

No message may become application-visible unless the verifier has accepted the session.

### 6.2 Speculative Non-Delivery

```text
forall s, m:
  State(s) = SPECULATIVE => not Deliver(s, m)
```

Speculative sessions may decrypt and buffer; they may not deliver.

### 6.3 Rollback Safety

```text
forall s, m:
  Abort(s) and Buffered(s, m) => always not Deliver(s, m)
```

Once a speculative session aborts, all isolated plaintext is destroyed and cannot later become application-visible.

### 6.4 Ordered Release

Let `<_s` be the protocol order for session `s`.

```text
VerifySuccess(s) and Buffered(s, m_i) and Buffered(s, m_j) and m_i <_s m_j
=> Deliver(s, m_i) happens before Deliver(s, m_j)
```

If a session verifies, isolated plaintext is released in protocol order.

### 6.5 State Continuity

```text
State(s) = SPECULATIVE and Recv(s, c_i) valid
=> ChannelStateAdvances(s)
```

This distinguishes SAC from ciphertext queuing. The protocol may keep cryptographic state synchronized during the verification window.

### 6.6 Semantic Non-Commitment

Let `Commit(s, m)` mean an application-visible semantic effect derived from plaintext `m`, including delivery, application acknowledgment, durable application write, command execution, notification, or application-level interpretation.

```text
State(s) = SPECULATIVE and Decrypt(s, c) -> m
=> not Commit(s, m)
```

This is stronger than delaying a callback by convention. It requires the channel implementation to define an application boundary and to make all semantic effects reachable only from `VERIFIED`.

## 7. Adversary Model

The adversary controls the network and may:

- deliver chosen ciphertexts;
- replay, reorder, delay, and drop messages;
- delay verifier responses;
- cause stale cached evidence to be used;
- attempt verifier equivocation through the trust source;
- attempt resource exhaustion against DIG;
- observe timing, traffic shape, aborts, and resource pressure;
- reveal selected ratchet states, subject to freshness rules;
- corrupt long-term keys outside the challenge freshness conditions.

The adversary does not break the cryptographic assumptions of the concrete instantiation, except with the stated advantages.

## 8. Delivery-Safety Game

The delivery-safety game captures the central SAC property.

1. The challenger initializes honest participants and a verifier.
2. The adversary controls network scheduling and may issue `Send`, `DelayVerify`, `RevealState`, and allowed `Corrupt` queries.
3. The adversary wins if an honest participant executes `Deliver(s, m)` while `Authorized(s)` is false.

The adversary's advantage is:

```text
Adv_delivery(A) = Pr[A wins delivery-safety game]
```

## 9. Proof Sketch for Delivery Safety

In the real game, `Deliver` can be reached only through the state transition system.

By construction:

```text
State = IDLE        => no channel delivery exists
State = SPECULATIVE => Decrypt may occur, Deliver may not occur
State = ABORTED     => buffer cleared, Deliver disabled
State = VERIFIED    => Deliver enabled
```

The only transition into `VERIFIED` is `VerifySuccess`, which requires verifier acceptance. Therefore:

```text
Deliver(s, m)
=> State(s) = VERIFIED
=> VerifySuccess(s)
=> Authorized(s)
```

An adversary can violate delivery safety only through one of three paths:

1. The verifier accepts false evidence.
2. The adversary forges cryptographic evidence required by the instantiation.
3. The implementation violates the DIG state machine.

Thus:

```text
Adv_delivery(A)
<= Pr[VerifierSafetyFailure]
 + Adv_crypto_forge(A)
 + Pr[DIGImplementationFailure]
```

## 10. Confidentiality Game

SAC itself is not a concrete cryptographic protocol. Confidentiality is proven for an instantiation. The generic structure is:

1. The challenger establishes a fresh session.
2. The adversary chooses equal-length messages `m0`, `m1`.
3. The challenger encrypts `mb` for random bit `b`.
4. The adversary wins by guessing `b`.

For an instantiation with a KEM, KDF, ratchet, and AEAD:

```text
Adv_conf(A)
<= Adv_KEM(A1)
 + Adv_KDF(A2)
 + Adv_AEAD(A3)
```

Atrium instantiates this with ML-KEM-768, HMAC-SHA3-384, and AES-256-GCM.

## 11. Ratchet Recovery Property

Let `Compromise(s, e)` reveal all symmetric state for session `s` in epoch `e`. Let `Refresh(s, e+1)` be an uncompromised entropy injection event.

Post-compromise recovery after refresh is:

```text
Compromise(s, e) and Refresh(s, e+1) and not Corrupt(KEM private key)
=> messages in epochs > e are confidential
```

This property is not inherent to SAC. It belongs to instantiations that define an uncompromised fresh entropy source.

## 12. Modeling Limits

This model does not prove:

- metadata privacy;
- absence of timing side channels;
- absence of resource-observable speculation;
- consensus liveness;
- endpoint security after application delivery;
- correctness of a concrete verifier;
- memory safety of an implementation.

These must be addressed by concrete instantiation specifications, implementation tests, and evaluation.
