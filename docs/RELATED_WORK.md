# Related Work and Positioning

Status: research positioning draft

Related documents:

- [ABSTRACTION.md](ABSTRACTION.md) defines Speculative Authenticated Channels (SAC).
- [FORMAL_MODEL.md](FORMAL_MODEL.md) states the delivery-safety model.
- [SPECIFICATION.md](SPECIFICATION.md) defines Atrium as a PQ/DID instantiation.
- [EVALUATION_PLAN.md](EVALUATION_PLAN.md) defines the experimental baselines.

## 1. Positioning

SAC should be positioned as a channel abstraction for delayed authorization environments. The main contribution is not a new KEM, signature scheme, ratchet, DID method, or transport format. The contribution is a delivery semantics:

```text
speculative cryptographic progression
without speculative semantic commitment
```

This separates SAC from three simpler designs. A strict design waits for authorization before decrypting, preserving safety but exposing verifier latency. A naive optimistic design decrypts and delivers immediately, preserving transport progress but allowing dirty delivery. A ciphertext queue delays all cryptographic processing, preserving safety but losing ratchet continuity, replay-state continuity, and early cryptographic failure detection. SAC occupies the middle point: it permits protocol state to progress while the application boundary remains closed.

## 2. Secure Channel Abstractions

TLS 1.3 defines a widely deployed authenticated secure channel with handshake-derived traffic secrets and an AEAD-protected record layer. Its record layer goals include confidentiality, integrity, non-replay/order protection, length concealment, and forward secrecy after key update. TLS assumes the channel's authentication state is resolved by the handshake before ordinary application delivery. SAC differs by modeling environments where the transport can establish cryptographic state before fresh authorization evidence has converged.

Augmented Secure Channels (ASC) study the security goal of the TLS 1.3 record layer as a channel abstraction with privacy-protected and authenticity-protected message components. Constructive cryptography and universally composable security provide the broader research style of specifying idealized resources and proving that protocols construct them. SAC is aligned with that style: it is not only a wire protocol, but a proposed resource with explicit `Decrypt`, `Deliver`, and application-visibility semantics.

Relevant anchors:

- RFC 8446, "The Transport Layer Security (TLS) Protocol Version 1.3": https://www.rfc-editor.org/rfc/rfc8446
- Badertscher et al., "Augmented Secure Channels and the Goal of the TLS 1.3 Record Layer": https://crypto.ethz.ch/publications/BMMRT15a.html
- Maurer and Renner, "Constructive Cryptography - A New Paradigm for Security Definitions and Proofs": https://crypto.ethz.ch/publications/Maurer11.html
- Canetti, "Universally Composable Security: A New Paradigm for Cryptographic Protocols": https://research.ibm.com/publications/universally-composable-security-a-new-paradigm-for-cryptographic-protocols

## 3. Secure Messaging and Ratchets

Signal's X3DH and Double Ratchet protocols are central references for asynchronous secure messaging. X3DH establishes a shared secret in an asynchronous setting, and the Double Ratchet advances per-message keys to provide forward secrecy and post-compromise recovery properties. MLS extends secure messaging to groups using epochs, authenticated membership, and tree-based key evolution.

SAC does not replace these mechanisms. Instead, it addresses a different semantic gap: what should a channel do when cryptographic processing can continue but the freshest identity, membership, attestation, or authorization evidence is delayed? Ratchets motivate why ciphertext queuing is insufficient: delaying all decryption also delays replay checks, skipped-message accounting, epoch handling, and key-state evolution. SAC makes that distinction explicit.

Relevant anchors:

- Signal X3DH specification: https://signal.org/docs/specifications/x3dh/
- Signal Double Ratchet specification: https://signal.org/docs/specifications/doubleratchet/
- Alwen et al., "The Double Ratchet: Security Notions, Proofs, and Modularization for the Signal Protocol": https://eprint.iacr.org/2018/1037.pdf
- RFC 9420, "The Messaging Layer Security (MLS) Protocol": https://www.rfc-editor.org/rfc/rfc9420

## 4. Optimistic Systems and Delayed Commit

SAC borrows intuition from optimistic systems: work may proceed before all validation is complete, but externally visible commitment is delayed until validation succeeds. Optimistic concurrency control validates transactions before commit. Transactional memory separates speculative execution from committed memory effects. Distributed commit protocols make agreement and visibility explicit.

The difference is the security object. Database and memory systems protect consistency of shared state. SAC protects authenticated delivery semantics of a cryptographic channel. Its commit point is not a database write; it is application-visible plaintext delivery. This translation is the research opportunity: delayed-commit ideas become delivery-safety properties for authenticated channels.

Relevant anchors:

- Kung and Robinson, "On Optimistic Methods for Concurrency Control": https://db.cs.berkeley.edu/cs286/papers/occ-tods1981.pdf
- Herlihy and Moss, "Transactional Memory: Architectural Support for Lock-Free Data Structures": https://cs.brown.edu/~mph/HerlihyM93/herlihy93transactional.pdf

## 5. Speculative Execution and Side Channels

The word "speculative" must be scoped carefully. Hardware speculative execution shows that computation without architectural commitment can still leak through microarchitectural side channels. SAC therefore must not claim physical non-interference or absence of observable speculation. Its claim is narrower: speculative plaintext must not cross the protocol's application-visible semantic boundary before authorization.

This stance should remain explicit in the paper. Timing, memory pressure, traffic shape, backpressure, and abort observability are outside the core SAC safety claim unless a concrete deployment adds separate mitigations. The formal property is not "nothing leaks"; it is "unauthorized plaintext is not delivered or semantically committed."

Relevant anchors:

- Kocher et al., "Spectre Attacks: Exploiting Speculative Execution": https://arxiv.org/abs/1801.01203
- Bhattacharyya et al., "SMoTherSpectre: exploiting speculative execution through port contention": https://arxiv.org/abs/1903.01843

## 6. Decentralized Identity, Attestation, and Slow Verifiers

Atrium uses DID-anchored post-quantum identity as one deployment setting for SAC. Other delayed-verifier settings include remote attestation, key-transparency systems, certificate-transparency-like evidence, quorum-backed authorization, confidential-computing admission, and distributed inference schedulers. These systems share a structural problem: fresh authorization may be slower than cryptographic channel establishment.

The research paper should keep this category as motivation and instantiation space, not as the central novelty. The central novelty remains the channel semantics that decouple authorization freshness from cryptographic continuity.

## 7. Claim Boundary

The paper should avoid claiming that SAC is a new universal secure transport or a replacement for TLS, Signal, or MLS. A stronger and safer claim is:

```text
SAC defines delivery-safe authenticated channel semantics
for systems where fresh authorization is delayed.
```

Atrium is then presented as a concrete PQ/DID instantiation used to evaluate whether SAC can reduce time-to-first-cryptographic-progress and preserve zero dirty delivery relative to strict and naive optimistic baselines.
