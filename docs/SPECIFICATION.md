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

### 1.1 Problem Statement

Classical authenticated key exchange protocols assume that peer authentication can be checked locally and quickly. This assumption does not hold in decentralized identity systems where the current public keys, revocation state, or authorization proof may depend on a slow trust anchor such as a blockchain, replicated registry, quorum certificate, or other eventually consistent verification service. A strict design that waits for fresh trust-anchor verification before creating a protected channel preserves safety, but it turns every cache miss or key-rotation event into a user-visible latency cliff. An optimistic design that uses cached identity material immediately can hide this latency, but it risks delivering plaintext generated under stale or revoked credentials.

Atrium addresses this tension by separating cryptographic establishment from application delivery. A peer may establish provisional encryption state with cached DID material and may process encrypted traffic while fresh verification runs in the background. The protocol does not, however, treat provisional decryption as authorization to affect the application. Decrypted plaintext remains isolated until the trust anchor confirms that the identity material used by the session is current. This design is intended for interactive messaging, IoT data exchange, and other applications where a short delay before application delivery is acceptable, but blocking the entire encrypted channel setup on a slow decentralized verifier is undesirable.

### 1.2 Design Goals

Atrium is designed around four goals. First, the protocol should be decentralized: peer identity is anchored in DIDs and verified against an external trust anchor rather than a centralized online certificate authority. Second, the data plane should be post-quantum resistant: session establishment uses ML-KEM, control authentication uses ML-DSA, and long sessions refresh post-quantum entropy through Epoch-KEM. Third, the protocol should be efficient and lightweight: ordinary data frames use a fixed 48-byte header, route by compact route IDs, and rely on AEAD authentication instead of per-message lattice signatures. Fourth, the protocol should be safe under speculation: any data decrypted before fresh identity verification succeeds must be prevented from reaching the application.

The protocol deliberately does not attempt to solve anonymous routing, consensus design, DID method standardization, group messaging, or application-level semantics. Those concerns may be layered above or below Atrium, but the core protocol is limited to establishing and maintaining a secure, DID-anchored channel with explicit speculative-delivery controls.

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

## 7. Protocol Operation

This section describes how the message types above compose into a complete Atrium session. It is intentionally written at the protocol level: an implementation may use different APIs or local storage structures, but it must preserve the ordering, authentication, state-transition, and delivery rules described here.

### 7.1 Route Binding

Before a relay can forward frames to a peer, it needs a compact routing key. A peer derives `route_id` from its DID and sends a `ROUTE_REGISTER` frame to the relay. In controlled test deployments this binding may be unauthenticated, but public deployments should authenticate route registration or use an admission policy, because a false route binding can redirect traffic. Once the binding exists, the relay only needs `to_route_id` from the fixed header to forward ordinary frames. The relay does not need to inspect `KEM_INIT`, `SECURE_MESSAGE`, or application payload contents to perform routing.

The route binding is not an anonymity mechanism. It avoids repeating full DID strings on every data frame and keeps the header fixed, but a relay that can guess a DID can compute its route ID. Applications requiring stronger metadata protection must add a separate private-routing layer.

### 7.2 Session Establishment

An initiator begins a session by resolving or loading the responder's DID material, deriving the responder's `route_id`, encapsulating an ML-KEM shared secret to the responder's KEM public key, and sending a credentialed `KEM_INIT` frame. The `KEM_INIT` payload carries the KEM ciphertext, the initiator nonce, the initiator DID, the responder DID, and any optional early-data ciphertext. The frame credential authenticates the initiator's control message and binds the handshake payload to the fixed header.

The responder decapsulates the ML-KEM ciphertext, verifies the credential, checks that the responder DID corresponds to the route and local identity, and replies with a credentialed `KEM_CONFIRM` frame. `KEM_CONFIRM` contributes the responder nonce and transcript hash. Both peers derive `session_id` from the transcript and derive the initial root secret and directional chain keys from the KEM shared secret and transcript context. The initial epoch is `epoch_id = 0`.

If fresh trust-anchor verification has already succeeded before application delivery is possible, the session may enter `VERIFIED`. If the handshake relies on cached DID material while fresh verification is still pending, the session enters `SPECULATIVE`. In both cases, cryptographic keys exist; the difference is whether decrypted application plaintext may be delivered.

### 7.3 Speculative Execution and Asynchronous Verification

When a session is established from cached DID material, both peers independently start asynchronous verification against the trust anchor. Verification checks that the DID document, verification methods, KEM public key, and relevant revocation or rotation state used by the session agree with the current trust-anchor evidence. The peer MUST NOT rely on an unauthenticated claim from the other side that verification has succeeded.

During this window, peers may send and receive `SECURE_MESSAGE` frames. Receivers may decrypt these frames to keep ratchets synchronized, but DIG prevents the decrypted plaintext from reaching the application. If verification succeeds, the peer emits an authenticated `VerificationStatus` with status `VERIFIED` when appropriate, upgrades local state, and releases isolated plaintext in order. If verification fails, the peer emits an authenticated `VerificationStatus` with status `ABORTED` when possible, clears speculative material, and closes or invalidates the session.

### 7.4 Protected Data Transfer

Ordinary application data is carried in `SECURE_MESSAGE` frames. Such frames do not carry `Credential`; their integrity and authenticity are provided by AEAD under message keys derived from the current directional ratchet. The complete fixed `Header` is used as AEAD associated data, which binds the encrypted payload to the session, epoch, sequence number, route, protocol suite, message type, and payload length.

The sender increments `sequence_number` for each outbound message in the current epoch. The receiver validates the epoch and sequence number before delivery decisions are made. Duplicate, decreasing, stale-epoch, or unknown-epoch messages are rejected. If the receiver is in `VERIFIED`, valid plaintext may be delivered immediately. If the receiver is in `SPECULATIVE`, valid plaintext is isolated under DIG.

### 7.5 Epoch Refresh

Long sessions use Epoch-KEM to inject fresh post-quantum entropy without paying the cost of ML-KEM on every message. When the entropy budget indicates that an epoch should be refreshed, a peer sends a credentialed `EPOCH_KEM` frame. This frame proposes `next_epoch_id`, carries a fresh ML-KEM ciphertext, and binds the refresh to the current epoch transcript. After both peers complete the refresh, they mix the new KEM shared secret into the current root state, derive new directional chain keys, reset per-direction sequence counters, and move to the new epoch.

An implementation must ensure that messages encrypted under different epochs are never processed with the same chain state. During an epoch transition, a receiver may either reject old-epoch messages or maintain a narrow, explicitly bounded replay window. Atrium v0.2 specifies the simpler behavior: unknown or stale epoch messages are rejected unless a future extension defines a replay window.

### 7.6 Failure and Rollback

Atrium treats failed verification and protocol failure as explicit state transitions rather than as recoverable application events. If trust-anchor verification fails, the affected session moves to `ABORTED`; speculative plaintext is destroyed and never delivered. If frame parsing, credential verification, transcript validation, AEAD authentication, sequence validation, or epoch validation fails fatally, the receiver also aborts the session. A peer should send an authenticated failure notice when possible, but local abort never depends on successful delivery of that notice.

This rollback rule is what makes speculation safe: an attacker may delay verification or cause a peer to perform work under stale cached material, but the attacker should not be able to convert that speculative work into application-visible effects unless the trust-anchor evidence eventually validates the session.

## 8. Wire Encoding

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

## 9. Canonical Encoding

Atrium frames are authenticated over bytes, not over abstract objects. Every conforming implementation must therefore produce the same byte sequence for the same credentialed control message. This section defines the canonical encoding rules for the base protocol. Alternative encodings may be used only if a future extension gives them a distinct `suite_id` or extension flag and specifies equivalent canonicalization rules.

All unsigned integers are encoded in network byte order. `uint8`, `uint16`, and `uint32` occupy exactly 1, 2, and 4 bytes respectively. Fixed-size byte strings are encoded as their raw bytes with no length prefix. Variable-size opaque byte strings are encoded as `uint16 length || bytes`, where `length` is the number of following bytes. Variable-size UTF-8 strings are encoded as `uint16 length || utf8-bytes`; the length is measured in bytes, not characters. Encoders MUST reject strings that are not valid UTF-8.

Payload fields are encoded in the exact order listed in Section 6 for the corresponding message type. Required fields MUST appear exactly once. Optional fields are encoded as a one-byte presence flag followed by the field value only when the flag is `1`. Unknown fields are not permitted in the base protocol payload encoding. A receiver MUST reject any payload that contains trailing bytes, out-of-order fields, duplicate fields, non-minimal length encodings, or an unknown extension bit.

DID strings used to derive `route_id` or `session_id` MUST be canonicalized before hashing. The base canonical form is the exact UTF-8 DID string after validating the DID syntax, preserving method-specific identifiers as supplied by the DID method. Implementations MUST NOT apply local aliases, display normalization, Unicode compatibility folding, percent-decoding, or case rewriting unless the DID method explicitly defines those transformations as canonical. The route ID is derived from the canonical DID bytes.

The canonical signed input for any credentialed frame is:

```text
Header || Payload
```

The credential itself is never part of the signed input. The AEAD associated data for a `SECURE_MESSAGE` is exactly the 48-byte `Header`. These two rules are interoperability requirements: an implementation that signs a parsed object, omits header fields, serializes optional fields differently, or authenticates a reconstructed header is not conforming.

## 10. State Machine

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

The following event-driven table is normative. If multiple rows match an event, the implementation MUST apply the first row whose condition is satisfied. Any event not listed for the current state MUST be treated as `PROTOCOL_ERROR`; if processing the event could affect key state or application delivery, the session MUST abort.

| Current State | Event | Condition | Action | Next State |
| --- | --- | --- | --- | --- |
| IDLE | local_start_session | fresh trust-anchor evidence already available | send or process `KEM_INIT`; derive root and chains | VERIFIED |
| IDLE | local_start_session | cached DID material available, fresh evidence pending | send `KEM_INIT`; derive provisional root and chains; start verification | SPECULATIVE |
| IDLE | recv(KEM_INIT) | credential valid, KEM decapsulation succeeds, fresh evidence available | send `KEM_CONFIRM`; derive root and chains | VERIFIED |
| IDLE | recv(KEM_INIT) | credential valid, KEM decapsulation succeeds, fresh evidence pending | send `KEM_CONFIRM`; derive provisional root and chains; start verification | SPECULATIVE |
| IDLE | recv(SECURE_MESSAGE) | any | send or record `PROTOCOL_ERROR`; do not decrypt | IDLE |
| SPECULATIVE | recv(SECURE_MESSAGE) | AEAD valid, epoch and sequence valid, buffer below limit | advance ratchet; push plaintext to DIG buffer | SPECULATIVE |
| SPECULATIVE | recv(SECURE_MESSAGE) | replay, stale epoch, unknown epoch, or AEAD failure | send fatal error if authenticated context permits; clear keys and buffer | ABORTED |
| SPECULATIVE | verification_success | trust-anchor evidence validates session identity material | atomically freeze DIG classification; flush buffer in order | VERIFIED |
| SPECULATIVE | verification_failure | trust-anchor evidence rejects or supersedes cached material | send `VerificationStatus(ABORTED)` when possible; clear keys and buffer | ABORTED |
| SPECULATIVE | verification_timeout | no evidence before local timeout | apply local policy: abort by default, or continue only if profile explicitly permits | ABORTED by default |
| SPECULATIVE | buffer_limit_exceeded | message count or byte limit exceeded | clear keys and buffer; close transport | ABORTED |
| SPECULATIVE | recv(VerificationStatus=ABORTED) | credential valid and peer matches session | clear keys and buffer | ABORTED |
| VERIFIED | recv(SECURE_MESSAGE) | AEAD valid, epoch and sequence valid | advance ratchet; deliver plaintext | VERIFIED |
| VERIFIED | recv(SECURE_MESSAGE) | replay, stale epoch, unknown epoch, or AEAD failure | send fatal error if appropriate; clear keys | ABORTED |
| VERIFIED | entropy_budget_exceeded | local budget threshold exceeded | initiate `EPOCH_KEM` | VERIFIED |
| VERIFIED | epoch_kem_complete | credential valid, transcript valid, `next_epoch_id` correct | mix epoch secret; reset counters; advance epoch | VERIFIED |
| VERIFIED | recv(VerificationStatus=ABORTED) | credential valid and peer matches session | clear keys and close transport | ABORTED |
| ABORTED | any frame | any | discard frame; do not deliver plaintext | ABORTED |

Timeouts are local policy parameters, but implementations MUST define them. The base profile requires a finite verification timeout for `SPECULATIVE`; an implementation that waits forever under delayed verification is vulnerable to proof-starvation resource attacks.

## 11. Data Isolation Gate

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

### 11.1 DIG Resource Bounds

DIG is a protocol-level resource boundary, not an unbounded application queue. Every implementation MUST define limits for speculative memory and speculative time. These limits are part of the security posture because an adversary can deliberately delay verification while sending valid ciphertext in order to consume memory.

The base profile defines the following default bounds:

| Parameter | Default | Requirement |
| --- | --- | --- |
| `max_speculative_messages` | 64 messages | Receiver MUST abort when the number of isolated plaintext records exceeds this value. |
| `max_speculative_bytes` | 1 MiB | Receiver MUST abort when total isolated plaintext bytes exceed this value. |
| `max_speculative_duration` | 30 seconds | Receiver MUST abort if verification has not completed within this interval, unless a stricter application profile overrides it. |
| `max_reorder_gap` | 0 | Base profile does not permit out-of-order speculative delivery; future extensions may define a bounded gap. |

Implementations MAY choose stricter limits. Implementations MUST NOT choose unbounded limits. When any DIG bound is exceeded, the session transitions to `ABORTED`, all isolated plaintext is destroyed, and no buffered data is delivered. This rule is intentionally harsh: continuing after memory pressure would let an attacker turn speculation into a resource exhaustion channel.

### 11.2 DIG Ordering and Discard Semantics

The base profile requires in-order release by `(epoch_id, sequence_number)`. Since stale epochs are rejected and the base profile does not allow a reorder window, the isolation buffer is a linear sequence for the active epoch. A receiver MUST reject duplicate sequence numbers before inserting plaintext into DIG. If a gap is observed, the receiver MUST either wait within its finite speculative bounds or abort; it MUST NOT release later plaintext before earlier plaintext during the `SPECULATIVE -> VERIFIED` transition.

On verification failure, timeout, buffer overflow, authenticated abort, or fatal protocol error, the receiver MUST discard the entire isolation buffer. Partial rollback is not allowed. A frame that has been decrypted but not released remains speculative; it has no application-visible status and must be treated as nonexistent after abort.

### 11.3 Side-Effect Prevention Model

The protocol boundary for DIG is the application delivery interface. Before `VERIFIED`, a conforming implementation MUST NOT expose speculative plaintext through callbacks, channels, logs, metrics labels, database writes, user notifications, command execution, application acknowledgments, or any other side-effecting interface. Implementations may record aggregate counters such as buffered byte counts or abort reasons, but they MUST NOT record speculative plaintext or application-derived interpretation of speculative plaintext.

Applications using Atrium MUST treat delivery from the protocol stack as the first point at which plaintext exists for application semantics. If an implementation exposes a lower-level debugging API that can inspect isolated plaintext, that API is outside the protocol security boundary and MUST NOT be enabled in a conforming deployment profile.

## 12. Bidirectional Ratchet and Epoch-KEM

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

### 12.1 Epoch-KEM Trigger

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

### 12.2 Epoch Transition

After a successful Epoch-KEM, both peers mix the new ML-KEM shared secret into the existing root state and derive fresh directional chains. The old epoch remains necessary for replay accounting and diagnostics, but it MUST NOT be used for encrypting new application data after the transition completes.

```text
mixed_root = HMAC-SHA3-384(
  old_root,
  "atrium epoch mix" || new_kem_secret || transcript_hash || next_epoch_id
)
```

The peers derive fresh directional chains from `mixed_root`, reset per-direction sequence counters, set `epoch_id = next_epoch_id`, and reset the leakage budget.

If an adversary revealed a previous epoch's ratchet state, a successful Epoch-KEM using uncompromised peer KEM keys restores secrecy for later epochs under the ML-KEM assumption.

## 13. Error Handling

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

## 14. Security Rationale

Atrium v0.2 aims to support the following security properties under the threat model in Section 3. The arguments below are intentionally concise. They identify the reduction targets and the role of the state machine; a more formal game-based proof sketch appears in Appendix A.

### 14.1 Session Confidentiality

Session confidentiality follows from the ML-KEM shared secret, transcript-bound key derivation, and AEAD protection of data frames. In the initial handshake, an adversary that does not know the responder's KEM private key cannot distinguish the ML-KEM shared secret from random except with the advantage of breaking ML-KEM-768. The derived root and chain keys are produced from that secret and transcript context using domain-separated HMAC-SHA3-384. Ordinary application bytes are encrypted under AES-256-GCM with the complete `Header` as associated data. Therefore, modifying the header, replaying the frame into another session, changing the epoch, or changing the sequence number invalidates the AEAD check.

The resulting informal bound is:

```text
Adv_conf <= Adv_ML-KEM-768 + Adv_HMAC-SHA3-384 + Adv_AES-256-GCM
```

### 14.2 Eventual Application-Layer Authentication

Atrium does not claim that every speculative channel is immediately authenticated against the freshest trust-anchor state. Instead, it claims that application delivery is authenticated before it becomes visible. In `SPECULATIVE`, decrypted plaintext is confined to the isolation buffer. The only transition that releases this buffer is `SPECULATIVE -> VERIFIED`, and that transition requires successful trust-anchor verification of the DID material used by the session. If the cached material is stale, revoked, or inconsistent with the trust anchor, the session transitions to `ABORTED` and the isolated plaintext is destroyed.

Thus a stale-cache attacker can at most cause speculative computation and buffering unless one of three events occurs: the implementation violates DIG, the adversary forges a required control-frame credential, or the trust anchor returns invalid evidence as valid. The resulting informal bound is:

```text
Pr[invalid Deliver] <= Pr[DIG implementation failure]
                    + Adv_ML-DSA-65
                    + Pr[trust-anchor safety failure]
```

### 14.3 Forward Secrecy Within an Epoch

Within an epoch, each message key is derived from the current chain key and the chain is immediately advanced using a separate domain. If HMAC-SHA3-384 behaves as a one-way PRF, compromise of a later chain key does not reveal earlier chain keys or earlier message keys. This gives per-epoch forward secrecy for messages sent before the state compromise, assuming old chain keys and message keys have been erased.

### 14.4 Post-Compromise Recovery After Epoch-KEM

If a ratchet state is compromised at epoch `e`, a purely symmetric ratchet cannot by itself recover secrecy against an adversary that continues to track chain evolution. Atrium restores secrecy by completing a later Epoch-KEM using uncompromised KEM private keys. The new ML-KEM shared secret is mixed into the root state and fresh directional chains are derived. Under the ML-KEM assumption, an adversary that lacks the KEM private key cannot derive the new epoch root from the compromised old state alone. Messages in epochs after the successful refresh therefore regain secrecy, subject to correct erasure and endpoint compromise assumptions.

### 14.5 Replay and Cross-Context Protection

Replay and cross-context attacks are constrained by binding protocol metadata into either signatures or AEAD associated data. Credentialed control frames sign `Header || Payload`, which binds message type, route, session, epoch, payload length, suite, and protocol code. Data frames use the complete `Header` as AEAD associated data. Since `(session_id, epoch_id, direction, sequence_number)` identifies each protected message, a receiver can reject duplicates and decreasing sequence numbers without decrypting them into a different context.

## 15. Security and Operational Considerations

This section describes operational rules that are not part of the frame syntax but are required for secure deployments and reproducible implementations. Profiles may choose stricter values, but they must not weaken the safety properties defined earlier in the specification.

### 15.1 Timeouts and Proof Starvation

An implementation MUST define a finite timeout for asynchronous trust-anchor verification. The base profile uses `max_speculative_duration = 30 seconds`. If verification does not complete within this interval, the default action is to abort the session and discard the isolation buffer. A deployment may choose a longer timeout only if it also defines proportional memory limits and denial-of-service controls. A deployment MUST NOT wait indefinitely in `SPECULATIVE`.

### 15.2 Cache TTL and Key Rotation

Cached DID material is a latency optimization, not an authority. Every cache entry SHOULD have a TTL and a version or evidence digest if the trust anchor provides one. A peer MAY use expired cache material to enter `SPECULATIVE`, but it MUST start fresh verification immediately. When verification reports that a DID document has rotated, expired, or been revoked, all sessions established from the stale material MUST abort unless the trust anchor explicitly proves continuity for the keys used by those sessions.

### 15.3 Clock Assumptions

The base protocol does not rely on synchronized clocks for cryptographic safety. Local clocks are used only for timeouts, telemetry, cache expiration, and operational policy. Implementations MUST NOT accept or reject a peer solely because of a peer-supplied wall-clock timestamp unless a future extension defines timestamp authentication and clock-skew rules.

### 15.4 Randomness Requirements

Implementations MUST use a cryptographically secure random number generator for ML-KEM encapsulation randomness, nonces, and any local secrets. AES-GCM nonces MUST be unique under a given AEAD key. If an implementation cannot guarantee random nonce uniqueness, it MUST derive nonces deterministically from `(session_id, epoch_id, direction, sequence_number)` with a domain-separated PRF. Reusing an AES-GCM nonce with the same key is a fatal implementation error.

### 15.5 Route Registration and Relay Policy

Relays route frames by `to_route_id`. A relay that accepts unauthenticated `ROUTE_REGISTER` frames is suitable only for local testing or trusted deployments. Public relays SHOULD require authenticated route registration, proof of DID control, or another admission policy. Relays SHOULD rate-limit route registration, failed route lookup, and malformed frames. A relay MUST NOT modify the header or payload of frames it forwards.

### 15.6 Logging and Telemetry

Implementations SHOULD expose telemetry for TTFB, verification convergence latency, DIG buffer occupancy, abort reasons, Epoch-KEM counts, and frame parse failures. Implementations MUST NOT log speculative plaintext, message keys, chain keys, ML-KEM shared secrets, or raw private key material. Logs MAY include route IDs and session IDs, but operators should treat them as metadata that may be linkable.

### 15.7 Downgrade and Extension Handling

Unknown `suite_id` values MUST be rejected. Unknown nonzero `extension_flags` MUST be rejected unless a negotiated extension explicitly defines them. Because `suite_id` and `extension_flags` are inside the signed control-frame input and AEAD associated data, attempts to rewrite them are detected by credential verification or AEAD authentication. Implementations MUST NOT silently fall back to a weaker suite.

### 15.8 Denial-of-Service Controls

Implementations SHOULD reject malformed frames before allocating large buffers. Receivers SHOULD enforce per-connection limits for in-flight handshakes, speculative sessions, failed credentials, and route misses. DIG bounds are mandatory, but they are not sufficient by themselves; relays and endpoints also need rate limits for unauthenticated traffic and computationally expensive control frames.

### 15.9 Reproducible Evaluation Methodology

Atrium evaluations SHOULD separate cryptographic cost, network transport cost, and trust-anchor verification latency. At minimum, an evaluation profile should report Time-to-First-Byte, verification convergence latency, dirty-delivery rate, DIG buffer occupancy, abort rate, Epoch-KEM frequency, and bytes sent per session. Experiments comparing Atrium to strict synchronous DID verification or asynchronous stale-cache designs MUST use the same trust-anchor latency distribution, network topology, payload sizes, cache state, and key-rotation schedule across all protocols.

Reported latency results SHOULD include sample count, median, p95, p99, and confidence intervals or bootstrapped error bars. Security experiments involving stale keys or malicious cache entries SHOULD report both attempted invalid sessions and actual invalid application deliveries. A result that only reports successful handshakes is insufficient to evaluate DIG.

## 16. Implementation Requirements

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

## 17. Non-goals and Limitations

Atrium v0.2 does not provide:

- Metadata privacy.
- Anonymous routing.
- Consensus protocol design.
- A production DID method.
- A general-purpose messaging application protocol.
- Multi-device group messaging.
- Complete formal verification of the implementation.

The current research artifact is intended to evaluate whether speculative cryptographic execution plus DIG can hide decentralized trust-anchor latency without allowing dirty application delivery.

## 18. Interoperability Test Vectors

This section provides minimal deterministic vectors for independent implementations. These vectors do not test ML-KEM, ML-DSA, or AEAD correctness; they test canonical byte layout, identifier derivation, and header construction. Cryptographic test vectors for the underlying primitives should be taken from their respective standards or library conformance suites.

### 18.1 route_id Derivation

For:

```text
canonical_did = "did:atrium:alice"
route_id = SHA3-256("atrium-v0.2 route" || canonical_did)[:16]
```

the expected route ID is:

```text
91bf35d07823ad3b8df87b1ccb3d7363
```

For:

```text
canonical_did = "did:atrium:bob"
route_id = SHA3-256("atrium-v0.2 route" || canonical_did)[:16]
```

the expected route ID is:

```text
f85bd7822d0c9cfef8fcdbadc29c71fb
```

### 18.2 session_id Derivation

For this deterministic transcript:

```text
initiator_did = "did:atrium:alice"
responder_did = "did:atrium:bob"
initiator_nonce = 32 bytes of 0x00
responder_nonce = 32 bytes of 0x11
initial_kem_ciphertext = 1088 bytes of 0x22
algorithm_suite = 0x00
```

the session ID is:

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

expected session_id = 0e8f80b7edb6718311cec049253b3dea
```

### 18.3 Header Encoding

For a pre-session `KEM_INIT` frame sent to Bob with no payload in this layout test:

```text
magic = 0xa7
version = 0x02
message_type = KEM_INIT = 0x1
flags = HAS_CREDENTIAL | CONTROL_FRAME = 0x5
type_flags = 0x15
code = 0x00
suite_id = 0x00
extension_flags = 0x00
session_id = 16 bytes of 0x00
to_route_id = f85bd7822d0c9cfef8fcdbadc29c71fb
epoch_id = 0
sequence_number = 0
payload_length = 0
```

the 48-byte header is:

```text
a7021500000000000000000000000000000000000000f85bd7822d0c9cfef8fcdbadc29c71fb00000000000000000000
```

For a `SECURE_MESSAGE` frame in the test session above:

```text
message_type = SECURE_MESSAGE = 0x3
flags = ENCRYPTED_PAYLOAD = 0x2
type_flags = 0x32
session_id = 0e8f80b7edb6718311cec049253b3dea
to_route_id = f85bd7822d0c9cfef8fcdbadc29c71fb
epoch_id = 0
sequence_number = 1
payload_length = 48
```

the 48-byte header is:

```text
a702320000000e8f80b7edb6718311cec049253b3deaf85bd7822d0c9cfef8fcdbadc29c71fb00000000000000010030
```

Implementations should include these vectors in their conformance tests before attempting full handshake interoperability.

## Appendix A. Formal Security Proof Sketch

This appendix gives a compact formalization of the security argument. It is not a machine-checked proof. Its purpose is to identify the games and assumptions that a full paper proof should expand.

### A.1 Participants, Sessions, and Adversary

Let `P` be the set of protocol participants. A local protocol instance is written as `Pi(i, s)`, where `i` identifies the participant and `s` identifies a local session instance. A session has a peer DID, a `session_id`, an `epoch_id`, directional chain states, a local delivery state in `{IDLE, SPECULATIVE, VERIFIED, ABORTED}`, and an isolation buffer.

The adversary is a probabilistic polynomial-time algorithm that controls the network. It may issue `Send` queries to deliver chosen frames, `DelayResolve` queries to delay trust-anchor responses, `RevealState` queries to expose selected ratchet states, and `Corrupt` queries to expose long-term endpoint secrets outside the freshness conditions of a challenge. The adversary wins the confidentiality game by distinguishing challenge plaintexts, and wins the delivery-authentication game by causing a participant to execute `Deliver(m)` for a message that is not justified by a verified peer session.

### A.2 Freshness Conditions

A challenge session is fresh for confidentiality if the adversary has not corrupted the relevant KEM private key before the challenge epoch secret is established and has not revealed the challenge message key before the challenge is answered. A challenge session is fresh for delivery authentication if the adversary has not corrupted the claimed peer's control-signing key and the trust anchor satisfies its stated safety property for the relevant DID evidence.

State-reveal queries are allowed for ratchet analysis. Revealing a later chain state should not reveal earlier message keys inside the same epoch. Revealing an epoch state before a later uncompromised Epoch-KEM does not by itself reveal message keys after that refresh.

### A.3 Confidentiality Game

In Game 0, the adversary interacts with the real protocol and chooses two equal-length plaintexts for a fresh challenge message. The challenger encrypts one of them in a `SECURE_MESSAGE` frame and returns the frame. The adversary outputs a bit guessing which plaintext was encrypted.

Game 1 replaces the ML-KEM shared secret used for the challenge session or challenge epoch with a uniformly random value. The difference between Game 0 and Game 1 is bounded by the IND-CCA advantage against ML-KEM-768. Game 2 replaces HMAC-derived chain and message keys with random keys. The difference is bounded by the PRF advantage against HMAC-SHA3-384. Game 3 uses AES-256-GCM under random keys with fixed associated data. The adversary's remaining advantage is bounded by the AEAD confidentiality advantage, assuming nonce uniqueness. Therefore:

```text
Adv_conf(A) <= Adv_ML-KEM-768(A1)
             + Adv_HMAC-SHA3-384(A2)
             + Adv_AES-256-GCM(A3)
```

### A.4 Delivery-Authentication Game

In the delivery-authentication game, the adversary wins if an honest participant delivers plaintext `m` to the application while the corresponding peer identity material is not verified by the trust anchor for that session. Atrium's state machine restricts `Deliver(m)` to the `VERIFIED` state. In `SPECULATIVE`, the only permitted action for decrypted plaintext is insertion into the isolation buffer. In `ABORTED`, the buffer is destroyed.

Game 0 is the real delivery game. Game 1 aborts if the adversary forges a valid control-frame credential for an honest DID verification method. The difference is bounded by the EUF-CMA advantage against ML-DSA-65. In Game 1, any transition to `VERIFIED` must be supported by local trust-anchor verification rather than by a forged peer claim. Game 2 aborts if the trust anchor accepts false identity evidence. The difference is bounded by the trust anchor's safety failure probability. In Game 2, any plaintext delivered by an honest implementation must have passed through `SPECULATIVE -> VERIFIED`; stale-cache sessions that fail verification transition to `ABORTED` and clear their buffers. Therefore:

```text
Pr[invalid Deliver] <= Adv_ML-DSA-65(A1)
                    + Pr[trust-anchor safety failure]
                    + Pr[DIG implementation failure]
```

The last term is included because DIG is an implementation-enforced state-machine property. The protocol specification makes the required state transitions explicit, but a concrete implementation must still be tested or verified against them.

### A.5 Ratchet Security

For per-epoch forward secrecy, consider an adversary that obtains chain state at step `t`. Earlier message keys were derived from prior chain states using domain-separated HMAC and those old states were erased. Recovering a prior message key from the later chain state requires inverting or distinguishing HMAC-SHA3-384 from a secure PRF.

For post-compromise recovery, suppose the adversary obtains the full symmetric ratchet state in epoch `e` but does not obtain the peer's KEM private key used in a later Epoch-KEM. The new epoch root mixes the old root with a fresh ML-KEM shared secret. Replacing that ML-KEM secret with random changes the adversary's view only by the ML-KEM advantage. Once mixed, later chain keys are derived from material unknown to the adversary, so messages after the refresh regain confidentiality under the KEM and KDF assumptions.

### A.6 Limitations of the Proof Sketch

This proof sketch does not model side channels, denial-of-service exhaustion beyond the stated buffer bound, metadata privacy, endpoint compromise after delivery, or consensus-layer liveness. It assumes nonce uniqueness for AEAD, correct erasure of old chain states, correct canonical encoding for signed frames, and a trust anchor with an explicit safety bound. A full paper proof should state these assumptions as separate lemmas and connect them to the experimental implementation.
