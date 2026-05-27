# Atrium Protocol Specification

Version: 0.2.0 research draft
Status: research artifact specification
Wire format: Atrium fixed binary frame

## 1. Scope and Status

Atrium is a decentralized, DID-anchored, post-quantum authenticated key exchange protocol for research on low-latency secure channels over slow trust anchors.

This document is the normative protocol source for Atrium v0.2. Implementations may use any internal representation, but conforming network messages MUST obey the frame format, field constraints, state transitions, and cryptographic processing rules defined here.

Atrium v0.2 focuses on two mechanisms:

1. Speculative Authenticated Key Exchange (S-AKE), which allows peers to establish cryptographic state from cached DID material while trust-anchor verification runs asynchronously.
2. Data Isolation Gate (DIG), which prevents decrypted plaintext from reaching the application until identity verification succeeds.

Atrium v0.2 also defines an adaptive bidirectional post-quantum ratchet. The ratchet uses symmetric per-message key evolution inside an epoch and injects fresh ML-KEM entropy at epoch boundaries.

This document uses the terms MUST, MUST NOT, SHOULD, SHOULD NOT, and MAY in their standard normative sense.

## 2. Terminology

| Term | Meaning |
| --- | --- |
| Peer | An Atrium endpoint identified by a DID. |
| Trust anchor | The authoritative DID registry or proof system used to resolve current identity material. |
| Oracle | The implementation abstraction that resolves DIDs against the trust anchor. |
| Cached DID document | Locally stored DID material used before fresh trust-anchor verification completes. |
| S-AKE | Speculative authenticated key exchange using cached DID material. |
| DIG | Data Isolation Gate; the state-machine rule that blocks application delivery in speculative state. |
| Session | One end-to-end secure channel between two DIDs. |
| Epoch | One generation of key material inside a session. |
| Epoch-KEM | A KEM refresh that injects fresh post-quantum entropy and advances the epoch. |
| Application data | Opaque bytes carried by Atrium after encryption. Chat, IoT, RPC, and similar formats are application profiles. |

## 3. Threat Model

The adversary may:

- Observe, drop, delay, replay, reorder, and modify network packets.
- Control untrusted relays.
- Delay DID resolution responses.
- Cause peers to use stale cached DID documents.
- Trigger key rotation or revocation races at the trust anchor.
- Attempt to exhaust speculative isolation buffers.
- Reveal a session ratchet state at a chosen time for post-compromise analysis.

The adversary may not:

- Break ML-KEM-768 IND-CCA security.
- Forge Dilithium3 signatures.
- Break the AEAD confidentiality or integrity primitive.
- Forge a valid trust-anchor proof beyond the assumed failure probability of the underlying registry or consensus system.

Atrium v0.2 does not provide metadata privacy. Full DIDs are not repeated on ordinary data frames, but route IDs, timing, message sizes, and handshake identity material remain observable to the relevant network participants.

## 4. Cryptographic Algorithms

| Purpose | Algorithm | Requirement |
| --- | --- | --- |
| Control-frame signature | Dilithium3 / ML-DSA-65 | REQUIRED for v0.2 control authentication. |
| Key encapsulation | ML-KEM-768 | REQUIRED for initial KEM and Epoch-KEM. |
| Hash and transcript hash | SHA3-384 | REQUIRED. |
| KDF and chain evolution | HMAC-SHA3-384 with domain separation | REQUIRED. |
| Message encryption | AES-256-GCM | REQUIRED for the v0.2 base suite. |
| DID anchor signature | Deployment-specific | MAY be Ed25519 or another DID registry mechanism. It is distinct from control-frame signatures. |

Algorithm identifiers are encoded by the 8-bit `suite_id` in the fixed frame header. Implementations MUST reject downgrade attempts that alter negotiated algorithm identifiers.

## 5. Protocol Identifiers

Atrium uses four fixed-size identifiers in its wire format.

### 5.1 session_id

`session_id` identifies one secure session between two DIDs. It is not used for relay routing.

It SHOULD be derived from the initial handshake transcript rather than generated as an unrelated UUID.

Recommended derivation:

```text
session_id = SHA3-384(
  "atrium-v0.2 session" ||
  initiator_did ||
  responder_did ||
  initiator_nonce ||
  responder_nonce ||
  initial_kem_ciphertext ||
  algorithm_suite
)[:16]
```

`session_id` is exactly 16 bytes in the v0.2 wire format. All protected messages in a session MUST carry the same `session_id`.

### 5.2 epoch_id

`epoch_id` identifies one generation of key material inside a session.

The field is a 32-bit unsigned integer. The initial handshake creates epoch `0`, and every successful Epoch-KEM advances the session to the next integer value. Each epoch owns independent send and receive chain keys, so an implementation MUST treat the epoch as part of the key-selection context. A protected message MUST be processed only with the chain keys for its declared epoch; a receiver MUST reject any message whose epoch is unknown, stale, or inconsistent with the local session state.

### 5.3 sequence_number

`sequence_number` identifies one message within one epoch and one direction.

The field is a 32-bit unsigned integer. It starts at `1` for each direction in each epoch and increases monotonically. The tuple `(session_id, epoch_id, direction, sequence_number)` is the replay-detection identity for a protected message. A receiver MUST reject duplicate sequence numbers and MUST reject decreasing sequence numbers for the same session, epoch, and direction.

### 5.4 route_id

`route_id` is a fixed-size relay routing key derived from a DID:

```text
route_id = SHA3-256("atrium-v0.2 route" || canonical_did)[:16]
```

`route_id` is exactly 16 bytes. Relays SHOULD route by `route_id -> connection` rather than by full DID strings.

This reduces fixed-header size and avoids exposing full DIDs on every data frame. It does not provide strong anonymity: if a DID is guessable, a relay can compute candidate route IDs offline.

Full `from_did` and `to_did` values appear only in handshake or registration payloads and MUST be authenticated by the relevant control-frame credential.

## 6. Message Syntax

The following messages define Atrium protocol semantics. Atrium v0.2 uses a fixed binary header followed by a length-bounded payload and, for selected control frames, a fixed-size credential. The payload encoding is a matter of implementation profile, but credentialed frames MUST have a canonical byte representation because signatures are computed over the header and payload bytes.

For the tables below:

- "signed" means the field is covered by `Credential.signature` when the frame carries a credential.
- "encrypted" means the field is inside an AEAD ciphertext.
- "required" means a conforming sender MUST include the field and a conforming receiver MUST reject the message if it is absent or invalid.

### 6.1 Frame

`Frame` is the outer protocol container.

| Field | Type | Required | Length | Signed | Encrypted | Constraints |
| --- | --- | --- | --- | --- | --- | --- |
| header | Header | yes | 48 bytes | yes for credentialed frames; AEAD AD for encrypted data frames | no | MUST be present on every frame. |
| payload | one message body | yes | 0..65535 bytes | yes for credentialed frames | maybe | MUST match `message_type`. |
| credential | Credential | conditional | suite-defined fixed length | no | no | Present iff the `HAS_CREDENTIAL` flag is set. |

Credentialed frames sign `header || payload`. The credential bytes MUST NOT be included in the signed bytes.

### 6.2 Header

`Header` is a 48-byte fixed header. It is designed to let relays and receivers parse routing, sequencing, protocol status, and payload size without decoding the variable payload. All multi-byte integer fields are encoded in network byte order.

| Field | Type | Required | Length | Signed | Encrypted | Constraints |
| --- | --- | --- | --- | --- | --- | --- |
| magic | uint8 | yes | 1 byte | yes | no | MUST be `0xA7` for Atrium v0.2 frames. |
| version | uint8 | yes | 1 byte | yes | no | MUST be `0x02`. |
| type_flags | uint8 | yes | 1 byte | yes | no | High 4 bits are message type; low 4 bits are flags. |
| code | uint8 | yes | 1 byte | yes | no | Protocol response code. `0x00` means OK. |
| suite_id | uint8 | yes | 1 byte | yes | no | Bit-packed algorithm suite identifier. |
| extension_flags | uint8 | yes | 1 byte | yes | no | 8-bit extension bitmap. MUST be zero if no extension is negotiated. |
| session_id | bytes | yes | 16 bytes | yes | no | Transcript-derived session identifier. Zero during route registration or pre-session control frames. |
| to_route_id | bytes | yes | 16 bytes | yes | no | Relay routing key derived from the destination DID. |
| epoch_id | uint32 | yes | 4 bytes | yes | no | Active key epoch. Zero before a session exists. |
| sequence_number | uint32 | yes | 4 bytes | yes | no | Per-direction sequence number in the active epoch. Zero for non-sequenced control frames. |
| payload_length | uint16 | yes | 2 bytes | yes | no | Length of `payload`, excluding credential. |

The fixed layout is:

```text
offset  size  field
0       1     magic
1       1     version
2       1     type_flags
3       1     code
4       1     suite_id
5       1     extension_flags
6       16    session_id
22      16    to_route_id
38      4     epoch_id
42      4     sequence_number
46      2     payload_length
```

`payload_length` is limited to 65535 bytes. Larger application messages MUST be fragmented by an upper layer or by a future Atrium extension.

### 6.3 Message Types, Flags, Codes, and Suites

`type_flags` is decoded as:

```text
message_type = type_flags >> 4
flags        = type_flags & 0x0f
```

Message types:

| Value | Name | Credential | Notes |
| --- | --- | --- | --- |
| 0x0 | RESERVED | no | MUST NOT be sent. |
| 0x1 | KEM_INIT | yes | Starts an initial session. |
| 0x2 | KEM_CONFIRM | yes | Confirms handshake transcript. |
| 0x3 | SECURE_MESSAGE | no | AEAD-authenticated data frame. |
| 0x4 | EPOCH_KEM | yes | Refreshes epoch key material. |
| 0x5 | VERIFICATION_STATUS | yes | Reports asynchronous trust-anchor result. |
| 0x6 | ERROR | conditional | Credential required for fatal/authentication-relevant errors. |
| 0x7 | ROUTE_REGISTER | optional | Binds a route ID to a relay connection. |

Flag bits:

| Bit | Name | Meaning |
| --- | --- | --- |
| 0 | HAS_CREDENTIAL | A fixed-size credential follows the payload. |
| 1 | ENCRYPTED_PAYLOAD | Payload contains AEAD-protected bytes or encrypted subfields. |
| 2 | CONTROL_FRAME | Frame changes protocol or routing state. |
| 3 | RESERVED | MUST be zero in v0.2. |

Protocol response codes:

| Value | Name | Meaning |
| --- | --- | --- |
| 0x00 | OK | Normal frame. |
| 0x01 | PROTOCOL_ERROR | Malformed frame or invalid transition. |
| 0x02 | AUTH_FAILED | Signature or credential validation failed. |
| 0x03 | VERIFY_FAILED | Trust-anchor verification failed. |
| 0x04 | DECRYPT_FAILED | AEAD decryption failed. |
| 0x05 | REPLAY_DETECTED | Duplicate or decreasing sequence number. |
| 0x06 | EPOCH_MISMATCH | Unknown, stale, or unexpected epoch. |
| 0x07 | ROUTE_NOT_FOUND | Relay cannot resolve `to_route_id`. |
| 0x08 | RATE_LIMITED | Receiver or relay throttled the frame. |
| 0xff | FATAL | Fatal error. It is session-aborting only when carried by an authenticated control frame or triggered by local parsing failure. |

`suite_id` is an 8-bit bit-packed suite identifier:

```text
bits 7..6: KEM id
bits 5..4: signature id
bits 3..2: hash/KDF id
bits 1..0: AEAD id
```

Atrium v0.2 defines only:

```text
KEM id       0b00 = ML-KEM-768
signature id 0b00 = Dilithium3 / ML-DSA-65
hash/KDF id  0b00 = SHA3-384 / HMAC-SHA3-384
AEAD id      0b00 = AES-256-GCM
suite_id     0x00 = the complete v0.2 suite above
```

Unknown suite bits MUST cause frame rejection.

`extension_flags` is an 8-bit extension bitmap. In v0.2, all bits are reserved and MUST be zero unless a future extension explicitly defines them. Unknown nonzero extension bits MUST cause frame rejection.

### 6.4 Credential

`Credential` authenticates selected control frames and binds them to a DID verification method. Its length is fixed for the selected `suite_id`. This means the protocol does not require a variable-length credential parser: once the receiver has parsed the header and suite, it can compute the exact credential size.

| Field | Type | Required | Length | Signed | Encrypted | Constraints |
| --- | --- | --- | --- | --- | --- | --- |
| key_id | bytes | yes | 16 bytes | no | no | `SHA3-256(verification_method_id)[:16]`. |
| signature | bytes | yes | `SignatureSize(suite_id)` | no | no | MUST verify over `header || payload`. |

For `suite_id = 0x00`, the signature algorithm is ML-DSA-65, historically known as Dilithium3. The signature length is the fixed ML-DSA-65 signature size defined by the selected cryptographic library or standard profile. The specification intentionally refers to this symbolic size rather than hard-coding a decimal constant, so implementations cannot diverge because of a stale value in the prose.

`Credential` is REQUIRED for `KEM_INIT`, `KEM_CONFIRM`, `EPOCH_KEM`, and `VERIFICATION_STATUS`.

`Credential` MUST NOT be attached to ordinary `SECURE_MESSAGE` frames. Secure data frames are authenticated by AEAD, not by per-message Dilithium or ML-DSA signatures.

### 6.5 RouteRegister

| Field | Type | Required | Length | Signed | Encrypted | Constraints |
| --- | --- | --- | --- | --- | --- | --- |
| route_id | bytes | yes | 16 bytes | yes if credentialed | no | MUST equal `SHA3-256("atrium-v0.2 route" || canonical_did)[:16]`. |
| did | string | no | variable | yes if present | no | MAY be omitted by relay-private deployments; if present, MUST match `route_id`. |
| expires_at | uint32 | no | 4 bytes logical | yes if credentialed | no | Optional route binding expiry as Unix seconds. |

A relay MAY accept unauthenticated route registration in local test deployments. Public deployments SHOULD require credentials or an out-of-band admission policy.

After registration, relays route by `to_route_id` and need not inspect full DIDs in ordinary frames.

### 6.6 KEMInit

`KEMInit` starts an initial session. It carries the first ML-KEM ciphertext, both DID identities needed for trust-anchor verification, and optional early data encrypted under keys derived from the initial KEM secret. A sender MUST use this message only for session establishment; later post-quantum entropy refreshes use `EpochKEM`.

| Field | Type | Required | Length | Signed | Encrypted | Constraints |
| --- | --- | --- | --- | --- | --- | --- |
| kem_id | uint8 | yes | 1 byte | yes | no | MUST match the KEM bits in `suite_id`. |
| kem_ciphertext | bytes | yes | 1088 bytes | yes | no | MUST be an ML-KEM-768 ciphertext. |
| initiator_nonce | bytes | yes | 32 bytes | yes | no | MUST be freshly generated. |
| initiator_did | string | yes | variable | yes | no | Full DID used for credential and trust-anchor verification. |
| responder_did | string | yes | variable | yes | no | Full DID whose route ID appears in `to_route_id`. |
| early_data_ciphertext | SecureMessage | no | variable | yes | yes | If present, MUST be subject to DIG. |
| transcript_context | bytes | yes | variable | yes | no | MUST include version and algorithm suite context. |

If `KEMInit` uses cached DID material, the receiver enters `SPECULATIVE` unless fresh verification has already completed.

### 6.7 KEMConfirm

`KEMConfirm` confirms responder participation and binds the transcript.

| Field | Type | Required | Length | Signed | Encrypted | Constraints |
| --- | --- | --- | --- | --- | --- | --- |
| responder_nonce | bytes | yes | 32 bytes | yes | no | MUST be freshly generated. |
| responder_did | string | yes | variable | yes | no | Full responder DID used for credential and trust-anchor verification. |
| transcript_hash | bytes | yes | 48 bytes | yes | no | MUST be SHA3-384 over the negotiated transcript. |
| early_response_ciphertext | SecureMessage | no | variable | yes | yes | If present, MUST be subject to DIG. |

`transcript_hash` MUST cover both DIDs, both nonces, the KEM ciphertext, protocol version, and algorithm identifiers.

### 6.8 SecureMessage

`SecureMessage` carries opaque application data.

| Field | Type | Required | Length | Signed | Encrypted | Constraints |
| --- | --- | --- | --- | --- | --- | --- |
| aead_nonce | bytes | yes | 12 bytes for AES-GCM | no | no | MUST be unique for a given key. |
| ciphertext | bytes | yes | variable | no | yes | AEAD output over opaque application bytes. |

The plaintext inside `ciphertext` is opaque to Atrium. A chat message is an application profile, not a core protocol message.

AEAD associated data MUST include the complete `Header` bytes. The header therefore binds message type, code, suite, `session_id`, `to_route_id`, `epoch_id`, `sequence_number`, and `payload_length`.

`SECURE_MESSAGE` frames MUST NOT carry `Credential`.

Receivers in `SPECULATIVE` MUST decrypt valid `SecureMessage` frames only to maintain ratchet synchronization and MUST place plaintext in the isolation buffer.

### 6.9 EpochKEM

`EpochKEM` injects fresh post-quantum entropy into an existing session.

| Field | Type | Required | Length | Signed | Encrypted | Constraints |
| --- | --- | --- | --- | --- | --- | --- |
| next_epoch_id | uint32 | yes | 4 bytes logical | yes | no | MUST equal current epoch plus 1. |
| kem_id | uint8 | yes | 1 byte | yes | no | MUST match the KEM bits in `suite_id`. |
| kem_ciphertext | bytes | yes | 1088 bytes | yes | no | MUST be generated for the peer's current KEM public key. |
| ratchet_reason | uint8 | yes | 1 byte | yes | no | SHOULD explain why refresh occurred. |
| current_epoch_transcript_hash | bytes | yes | 48 bytes | yes | no | Binds refresh to current session and epoch. |

Valid `ratchet_reason` values are:

```text
0x01 ENTROPY_BUDGET_EXCEEDED
0x02 TIME_INTERVAL
0x03 MESSAGE_COUNT
0x04 MANUAL_REFRESH
0x05 POST_COMPROMISE_RECOVERY
```

After a successful Epoch-KEM, peers MUST derive fresh send and receive chain keys and advance to `next_epoch_id`.

### 6.10 VerificationStatus

`VerificationStatus` communicates asynchronous trust-anchor verification results.

| Field | Type | Required | Length | Signed | Encrypted | Constraints |
| --- | --- | --- | --- | --- | --- | --- |
| status | uint8 | yes | 1 byte | yes | no | `0x01 VERIFIED`, `0x02 ABORTED`. |
| proof_digest | bytes | no | 48 bytes recommended | yes | no | Digest of trust-anchor evidence, if available. |
| reason_code | uint8 | yes | 1 byte | yes | no | MUST explain abort or verification outcome. |

A peer receiving authenticated `ABORTED` due to verification failure MUST transition to `ABORTED`, clear session material, clear isolation buffers, and close the transport.

### 6.11 Error

`Error` carries protocol-level failures that are not trust-anchor verification outcomes.

| Field | Type | Required | Length | Signed | Encrypted | Constraints |
| --- | --- | --- | --- | --- | --- | --- |
| detail_code | uint8 | yes | 1 byte | yes if credentialed | no | MUST refine `Header.code`. |
| reason | string | yes | variable | yes | no | SHOULD be safe for logs. |
| fatal | bool | yes | 1 byte logical | yes | no | If true, receiver MUST abort the session. |

Verification success or failure SHOULD use `VerificationStatus`, not `Error`.

## 7. Wire Encoding

The v0.2 wire unit is an Atrium frame:

```text
Frame = Header || Payload || Optional Credential
```

`Header` is always exactly 48 bytes. `Payload` is exactly `payload_length` bytes. `Credential` is present iff the `HAS_CREDENTIAL` flag is set, and its length is determined by `suite_id`.

The payload is an encoded instance of the message body indicated by `message_type`. The base protocol requires that credentialed payloads have canonical bytes for signing and verification. Implementations that use a schema language, binary codec, or hand-written encoder MUST define one canonical serialization for each credentialed payload type.

The signed bytes for a credentialed frame are:

```text
Header || Payload
```

For `SECURE_MESSAGE`, the AEAD associated data is:

```text
Header
```

Plain TCP transports read frames by first reading 48 bytes, parsing `payload_length`, reading the payload, then reading the fixed credential length if `HAS_CREDENTIAL` is set.

## 8. State Machine

Atrium sessions have four states. These states are local security states, not statements that can be trusted merely because a peer placed a value in a frame. A peer may report success, failure, or verification status through authenticated control frames, but the receiving implementation decides delivery and key lifetime from its own state machine.

| State | Meaning | Delivery rule |
| --- | --- | --- |
| IDLE | No active session keys. | Application data MUST NOT be sent or delivered. |
| SPECULATIVE | KEM state exists, but fresh DID verification is pending. | Ciphertext MAY be processed; plaintext MUST be isolated. |
| VERIFIED | Trust-anchor verification succeeded. | Buffered plaintext is flushed, then new plaintext is delivered immediately. |
| ABORTED | Verification failed or a fatal protocol error occurred. | Keys and buffers MUST be cleared; transport SHOULD close. |

Allowed transitions:

| Transition | Trigger | Required action |
| --- | --- | --- |
| IDLE -> SPECULATIVE | Cached DID material is used for KEM before fresh verification completes. | Initialize chains and start asynchronous verification. |
| IDLE -> VERIFIED | Fresh DID verification completes before data delivery. | Initialize chains in verified mode. |
| SPECULATIVE -> VERIFIED | Fresh trust-anchor verification succeeds. | Atomically release isolation buffer in sequence order. |
| SPECULATIVE -> ABORTED | Verification fails, frame authentication fails, fatal decryption error occurs, or buffer limit is exceeded. | Clear keys, clear buffer, notify peer if possible, close transport. |
| VERIFIED -> ABORTED | Fatal protocol error or authenticated rollback signal. | Clear keys, close transport. |

The transition from `IDLE` to `SPECULATIVE` is the latency-hiding path: the implementation has enough cached DID material to establish cryptographic state, but it has not yet obtained fresh trust-anchor evidence. The transition from `SPECULATIVE` to `VERIFIED` is the safety convergence point: only after this transition may isolated plaintext be released. The transition from `SPECULATIVE` to `ABORTED` is the rollback path: it discards speculative cryptographic progress when the trust anchor or protocol checks do not support delivery. Implementations MUST enforce these transitions locally even if a peer sends a conflicting control payload.

## 9. Data Isolation Gate

DIG is the mechanism that separates cryptographic progress from application-visible effects. Atrium permits a peer to perform speculative decryption because the ratchet state must remain synchronized with the network stream; however, speculative decryption alone is not evidence that the peer identity is current at the trust anchor. The protocol therefore treats decrypted plaintext as quarantined data until asynchronous verification proves that the cached DID material used for the session agrees with the trust anchor.

While a session is in `SPECULATIVE`, a receiver MAY decrypt valid ciphertext and advance the receive ratchet, but it MUST NOT invoke application callbacks, mutate application state, or otherwise expose decrypted plaintext outside the protocol boundary. The receiver MUST place decrypted plaintext into an isolation buffer indexed by `(epoch_id, sequence_number)`, and it MUST enforce a finite high-water mark for that buffer. If the buffer exceeds the configured limit, the session MUST transition to `ABORTED`; this transition is a denial-of-service defense and does not by itself imply authentication failure.

When asynchronous verification succeeds, the session transitions from `SPECULATIVE` to `VERIFIED`. This transition is the only point at which speculative plaintext may become application-visible. The implementation MUST make the transition atomic with respect to further receive processing: after the transition begins, no additional message may be classified under the old speculative delivery rule. The implementation MUST then release buffered plaintext in sequence order, and after the buffer has been released, subsequent valid plaintext MAY be delivered immediately.

When asynchronous verification fails, or when a fatal protocol violation occurs during speculation, the session transitions to `ABORTED`. In this state, the implementation MUST clear all isolated plaintext, clear session keys to the extent supported by the runtime, and close or otherwise invalidate the transport. No buffered plaintext may be delivered after an abort, even if it was successfully decrypted before the abort was triggered.

The normative DIG rules are:

- `SPECULATIVE` permits decryption and ratchet advancement, but forbids application delivery.
- Decrypted speculative plaintext MUST be stored only in the isolation buffer.
- The isolation buffer MUST preserve `(epoch_id, sequence_number)` order.
- The isolation buffer MUST have a configured high-water mark.
- `VERIFIED` is the only state that may release isolated plaintext.
- `ABORTED` MUST destroy isolated plaintext and MUST NOT release it.

## 10. Bidirectional Ratchet and Epoch-KEM

Each session epoch contains two independent symmetric chains:

```text
send_chain
recv_chain
```

The two chains prevent send-side and receive-side key evolution from interfering with each other. For each outgoing message, the sender derives a message key and advances `send_chain`. For each incoming message, the receiver derives the expected message key and advances `recv_chain`. This construction gives efficient per-message forward secrecy inside an epoch while avoiding the bandwidth and CPU cost of running ML-KEM for every message.

The reference chain construction is:

```text
message_key = HMAC-SHA3-384(chain_key, "atrium message key")
next_chain  = HMAC-SHA3-384(chain_key, "atrium next chain")
aead_key    = first_32_bytes(HMAC-SHA3-384(message_key, "atrium aes-256-gcm key"))
```

The old chain key MUST be discarded after deriving `next_chain`.

### 10.1 Epoch-KEM Trigger

Atrium maintains an accumulated leakage budget to decide when symmetric-only evolution should be refreshed with new post-quantum entropy. The model is intentionally a scheduling policy rather than a cryptographic assumption: it gives implementations a tunable way to trade bandwidth and CPU cost against long-session exposure.

```text
AccumulatedLeakage =
  lambda_t * seconds_since_last_kem +
  lambda_n * messages_since_last_kem
```

When:

```text
AccumulatedLeakage > C
```

the implementation SHOULD initiate Epoch-KEM.

This model is a scheduling policy for resource-aware post-quantum refresh. It is not, by itself, a proof of optimality.

### 10.2 Epoch Transition

After a successful Epoch-KEM, both peers mix the new ML-KEM shared secret into the existing root state and derive fresh directional chains. The old epoch remains necessary for replay accounting and diagnostics, but it MUST NOT be used for encrypting new application data after the transition completes.

```text
mixed_root = HMAC-SHA3-384(
  old_root,
  "atrium epoch mix" || new_kem_secret || transcript_hash || next_epoch_id
)
```

The peers derive fresh directional chains from `mixed_root`, reset per-direction sequence counters, set `epoch_id = next_epoch_id`, and reset the leakage budget.

If an adversary revealed a previous epoch's ratchet state, a successful Epoch-KEM using uncompromised peer KEM keys restores secrecy for later epochs under the ML-KEM assumption.

## 11. Error Handling

Error handling distinguishes local parsing failures, authenticated peer failures, and trust-anchor verification failures. Local parsing failures, such as malformed headers or impossible payload lengths, are handled immediately because they do not depend on peer authentication. Peer-supplied fatal errors affect session state only when they are carried by an authenticated control frame. Trust-anchor failures are represented by `VerificationStatus` and cause speculative state to be discarded.

An implementation MUST abort the session on the following conditions:

- Invalid control-frame signature for a message that requires authentication.
- Unknown critical extension.
- Invalid KEM ciphertext length.
- Invalid transcript hash.
- Duplicate or decreasing sequence number.
- Message for an unknown or stale epoch.
- Isolation buffer overflow.
- Authenticated verification failure.
- Authenticated fatal `Error` frame.

An implementation SHOULD attempt to send an authenticated `VerificationStatus{status=ABORTED}` or fatal `Error` before closing the transport. This notification is only a best-effort optimization to shorten the peer's risk window; local abort MUST NOT depend on successful delivery of the notice.

## 12. Security Claims

Atrium v0.2 aims to support these claims under the threat model in Section 3.

### 12.1 Session Confidentiality

If ML-KEM-768, the KDF, and AES-256-GCM remain secure, an adversary that does not compromise the relevant session state cannot distinguish protected application plaintexts from random strings beyond negligible advantage.

### 12.2 Eventual Application-Layer Authentication

In `SPECULATIVE`, decrypted plaintext is not delivered to the application. Therefore stale-cache attacks may create temporary cryptographic state, but they MUST NOT cause application-visible plaintext delivery before trust-anchor verification succeeds.

The probability of invalid application delivery is bounded by implementation correctness of DIG plus the failure probability of the trust anchor and control-frame authentication.

### 12.3 Forward Secrecy Within an Epoch

The one-way chain construction prevents recovery of earlier message keys from a later chain key, assuming HMAC-SHA3-384 behaves as a secure PRF.

### 12.4 Post-Compromise Recovery After Epoch-KEM

If a ratchet state is compromised at epoch `e`, and a later Epoch-KEM completes using uncompromised KEM private keys, messages in epochs after the refresh regain secrecy under the ML-KEM assumption.

## 13. Implementation Requirements

A conforming v0.2 implementation MUST:

- Verify frame credentials before using frames for state transitions.
- Derive `session_id` from the handshake transcript or provide an equally strong binding.
- Track `epoch_id` and per-direction sequence numbers.
- Use AEAD associated data covering protocol and routing metadata.
- Treat application payloads as opaque bytes.
- Enforce DIG in `SPECULATIVE`.
- Bound the isolation buffer.
- Abort on authenticated verification failure.
- Expose enough telemetry to measure TTFB, verification convergence latency, dirty-delivery rate, buffer memory, and Epoch-KEM overhead.

## 14. Non-goals and Limitations

Atrium v0.2 does not provide:

- Metadata privacy.
- Anonymous routing.
- Consensus protocol design.
- A production DID method.
- A general-purpose messaging application protocol.
- Multi-device group messaging.
- Complete formal verification of the implementation.

The current research artifact is intended to evaluate whether speculative cryptographic execution plus DIG can hide decentralized trust-anchor latency without allowing dirty application delivery.
