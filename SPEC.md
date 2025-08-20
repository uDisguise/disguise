# Disguise Protocol Specification

## 1\. Introduction

The **Disguise Protocol** is a transport-layer obfuscation and anti-censorship protocol designed to embed arbitrary data streams within a traffic pattern that imitates common web browsing behavior. Unlike protocols that emulate a single stream type, Disguise dynamically adapts to mimic multiple popular web protocols and services, such as HTTPS, HTTP/2, and QUIC, to evade passive and active traffic analysis.

Its primary goals are:

  - To resist deep packet inspection (DPI) and traffic analysis by generating a highly variable and randomized traffic profile that is statistically indistinguishable from legitimate web activity.
  - To provide advanced cover traffic mechanisms, including dynamic traffic simulation, adaptive burst shaping, and content-based padding, that are context-aware and responsive to network conditions.
  - To operate as a transparent layer over a secure transport like TLS or QUIC, ensuring all obfuscation occurs within the encrypted payload, invisible to middleboxes.

## 2\. Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119 [[RFC2119]](https://datatracker.ietf.org/doc/html/rfc2119).

## 3\. Protocol Overview

Disguise operates as a framing and obfuscation layer **above** the TLS/QUIC record layer but **below** the application payload. All obfuscation, fragmentation, and padding actions MUST occur before transport-level encryption and after decryption. This design ensures that all observable features on the network, including packet sizes, timing, and flow characteristics, are indistinguishable from a variety of web traffic patterns.

The protocol introduces a message framing, a flexible header, and dynamic fragmentation logic, all contained within the encrypted portion of a transport record.

-----

## 4\. Packet Format

Each Disguise protocol packet (a "cell") is structured as follows. The header is designed to be highly variable in size and content to mimic different traffic types.

```
0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Cell ID (2)  |   Type (1)    |   Flags (1)   | Seq (4)       |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Timestamp (8)                              |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|       PayloadLen (2)  |  PaddingLen (2) | RandOffset (2)      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                           Payload (variable)                  |
|                                                               |
+---------------------------------------------------------------+
|                      Padding (variable)                       |
+---------------------------------------------------------------+
```

### Field Definitions

  - **Cell ID (2 bytes, uint16, Big Endian):** Randomly generated ID to correlate related cells, similar to HTTP/2 streams. It is used to group fragmented payloads. Dummy cells use `0x0000`.
  - **Type (1 byte, uint8):** Defines the cell's purpose. Examples include `0x01` (Data), `0x02` (Handshake), `0x03` (Control), `0x04` (Dummy).
  - **Flags (1 byte, uint8):** Bitmask for control information, e.g., `0x01` (End of Stream), `0x02` (Urgent).
  - **Seq (4 bytes, uint32, Big Endian):** Monotonically increasing sequence number for reassembly of a single payload.
  - **Timestamp (8 bytes, int64, Big Endian):** Milliseconds since Unix epoch, providing a fine-grained timing reference for de-jittering.
  - **PayloadLen (2 bytes, uint16, Big Endian):** Length of the actual application payload.
  - **PaddingLen (2 bytes, uint16, Big Endian):** Length of the cryptographically random padding.
  - **RandOffset (2 bytes, uint16, Big Endian):** Random offset for the payload within the total cell size, designed to disrupt length-based analysis. Payload and padding can be reordered based on this offset.
  - **Payload:** Application data, up to `DisguiseMaxFrag` bytes per cell.
  - **Padding:** Cryptographically random bytes used to obscure the true payload length.

-----

## 5\. Fragmentation and Reassembly

  - **Fragmentation:** Application data is split into cells of random total length, adhering to a distribution that mimics common web traffic (e.g., small cells for HTTP headers, large cells for image data). The distribution is dynamically chosen from a predefined set of profiles (see Section 6).
  - **Padding:** Padding is not just random; it can be "content-aware." For example, dummy cells may be filled with data that mimics compressed HTTP/2 headers or base64-encoded strings to further blend in. Padding length is calculated as `TotalLen - HeaderLen - PayloadLen`.
  - **Reassembly:** The receiver uses the **Cell ID** and **Seq** fields to reassemble payloads. The `RandOffset` field allows for variable payload positioning within the cell, making it impossible to determine payload start from a fixed header length.

-----

## 6\. Cover Traffic and Adaptive Modes

Disguise's core innovation is its ability to simulate various network behaviors. The system learns the user's typical traffic patterns and replicates them.

  - **Dynamic Profiling:** The protocol maintains a library of traffic profiles (e.g., "Web Browsing," "Video Streaming," "Large File Download"). It uses machine learning models to analyze the user's real traffic and selects the most appropriate profile to emulate, dynamically changing packet sizes, timing, and burst characteristics.
  - **Adaptive Bursts:** Instead of simple, periodic bursts, Disguise uses a statistical model to generate bursts that match the timing and size distribution of common protocols like HTTP/2. Bursts are triggered based on real traffic events (e.g., a new connection) or a learned schedule.
  - **Active Probing Simulation:** Disguise MAY send small, seemingly random control cells (e.g., `Type: 0x03`) that mimic protocol-specific keep-alives or pings, making the connection appear "chatty" and non-idle.

-----

## 7\. Transmission Scheduling

  - Each cell is assigned a randomized send time within a learned delay distribution that matches the selected traffic profile.
  - The sender maintains a complex priority queue that not only considers send time but also the type of cell (e.g., real data vs. cover traffic) to prioritize delivery while maintaining the obfuscation.

-----

## 8\. Error Handling

  - The protocol MUST gracefully handle cell corruption, retransmission requests (if the underlying transport supports it), and out-of-order delivery.
  - Duplicate or out-of-window cells are dropped silently to avoid revealing network conditions.
  - All errors related to reassembly or cell decoding are logged.

-----

## 9\. Security Considerations

  - All randomness MUST be generated using a cryptographically secure PRNG.
  - The `RandOffset` and `PaddingLen` fields MUST be generated with high entropy to prevent leakage of the true payload size and position.
  - The sequence numbers and timestamps are designed to mitigate replay attacks.
  - The protocol's effectiveness relies on the underlying transport's security (e.g., TLS 1.3 or QUIC). Disguise only obfuscates the plaintext; it does not provide cryptographic security.

-----

## 10\. Negotiation and Compatibility

  - Both endpoints MUST agree to use the Disguise protocol. This negotiation is RECOMMENDED to occur via a custom TLS extension or ALPN (Application-Layer Protocol Negotiation).
  - The protocol is only active if enabled on both sides. If negotiation fails, the connection SHOULD fall back to standard TLS.

-----

## 11\. Parameter Summary

| Parameter          | Value (default) | Description                                                |
|:-------------------|:---------------:|:-----------------------------------------------------------|
| CellIDLen          | 2 bytes         | Length of the Cell ID                                      |
| HeaderLen          | 20 bytes        | Minimum header length                                      |
| MinCellSize        | 64 bytes        | Minimum total cell size                                    |
| MaxCellSize        | 1400 bytes      | Maximum total cell size, to fit within common MTUs         |
| ProfileSwitchDelay | 5 minutes       | Min interval to switch traffic simulation profiles         |
| LatencyJitter      | 20ms            | Max artificial delay for latency randomization             |
| ProbingInterval    | 15s             | Interval for sending dummy "keep-alive" cells              |
| EWMAAlpha          | 0.1             | Smoothing factor for traffic analysis                      |

-----

## 12\. API and Runtime Control

  - An API MUST be provided to set the desired traffic simulation profile at runtime.
  - Supported profiles include:
      - `DisguiseProfileOff` (no obfuscation)
      - `DisguiseProfileWeb` (mimics general web browsing)
      - `DisguiseProfileVideo` (mimics video streaming bursts)
      - `DisguiseProfileDynamic` (default, uses machine learning to adapt)
  - Changes to the profile take effect immediately and are applied to new cells.

-----

## 13\. IANA Considerations

This protocol does not require any IANA actions.

## 14\. References

  - [RFC2119] S. Bradner, "Key words for use in RFCs to Indicate Requirement Levels", BCP 14, RFC 2119, March 1997.
  - [TLS 1.3] RFC 8446: The Transport Layer Security (TLS) Protocol Version 1.3.

-----

**Author:** Yongkang Zhou 
**Status:** STANDARD  
**Intended status:** Experimental
