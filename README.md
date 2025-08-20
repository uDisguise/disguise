# Go TLS with Disguise: An Obfuscated TLS Implementation

This repository is a fork of the standard Go `crypto/tls` library, enhanced with a built-in traffic obfuscation layer. By integrating the Disguise protocol directly into the TLS handshake and record layer, this library provides secure, end-to-end encrypted communication that is also resilient to network traffic analysis and deep packet inspection (DPI).

The Disguise protocol operates transparently beneath the standard TLS record layer, shaping and padding network packets to mimic benign traffic patterns.

## Core Modifications

  - **Seamless Integration**: A `disguise.Manager` is now initialized and attached directly to the `tls.Conn` object. This manager is responsible for the entire lifecycle of the obfuscation protocol, including dynamic profiling and scheduling.
  - **Obfuscated I/O**: The `Read()` and `Write()` methods of the `tls.Conn` have been modified to use the Disguise protocol's queuing and scheduling logic.
      - `Write()` queues application data to the Disguise manager. The manager then fragments this data into cells, intelligently pads them, and sends them to the underlying network connection according to its dynamic schedule.
      - `Read()` reads incoming TLS records, passes them to the Disguise manager for reassembly, and returns the original application data to the caller once a complete message is received.
  - **Adaptive Behavior**: The built-in dynamic profiling mechanism is active by default, ensuring that the traffic pattern adapts to the network load, further enhancing its covertness.

## Installation

This is not a standard Go module that can be installed via `go get`. Since it modifies the standard library, you must use Go's `replace` directive to point to this modified version.

1.  Clone this repository to your local machine.
2.  In your `go.mod` file, add the `replace` directive pointing to your local path:

<!-- end list -->

```go
module my-project

go 1.21

require (
    // other dependencies
)

replace crypto/tls => github.com/uDisguise/disguise
```

## Usage

Using the modified TLS library is straightforward. For most use cases, your code will remain nearly identical to a standard TLS implementation. The obfuscation happens automatically behind the scenes.

Here is a basic client example demonstrating how to establish a connection and exchange data.

```go
package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	// --- Client Side ---
	fmt.Println("Attempting to dial TLS connection...")
	
	// Create a TLS configuration.
	// The Disguise protocol will be initialized internally with a default "Dynamic" profile.
	config := &tls.Config{
		InsecureSkipVerify: true, // For example purposes, do not use in production
	}

	// Establish a TLS connection to the server.
	// The modified Dial function will internally set up the Disguise manager.
	conn, err := tls.Dial("tcp", "example.com:443", config)
	if err != nil {
		log.Fatalf("Failed to dial TLS connection: %v", err)
	}
	defer conn.Close()
	
	fmt.Println("Successfully established TLS connection with Disguise enabled.")

	// Send some application data.
	// This data will be fragmented, shaped, and padded by the Disguise protocol
	// before being sent over the network.
	message := []byte("Hello from the disguised client!")
	_, err = conn.Write(message)
	if err != nil {
		log.Fatalf("Failed to write to connection: %v", err)
	}
	fmt.Printf("Wrote %d bytes of application data.\n", len(message))

	// In a real application, you would read the response here.
	// The underlying Disguise protocol will reassemble incoming packets
	// transparently.
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Fatalf("Failed to read from connection: %v", err)
	}
	fmt.Printf("Read %d bytes from server: %s\n", n, string(buffer[:n]))
}

// NOTE: A corresponding server using this same modified library is required.
```

## Customization

The current implementation initializes the `disguise.Manager` with a hardcoded `profile.Dynamic` profile, which automatically adapts to the network traffic.

To use a different profile (e.g., `profile.WebBrowsing` or `profile.VideoStreaming`), you would need to modify the `conn.go` file directly to pass the desired profile to `disguise.NewManager()`. A more flexible implementation would involve adding a new field to the `tls.Config` struct, but that requires more extensive changes to the standard library.

## Limitations & Future Work

  - **Server Requirement**: This library requires both the client and server to be running this same modified version to correctly interpret the Disguised traffic.
  - **Development Status**: This is a proof-of-concept implementation. It is not battle-tested and may have undiscovered bugs or performance issues.
  - **Single-Stream Reassembly**: The current reassembler handles only a single logical stream. For multiplexed connections (e.g., HTTP/2), a more complex reassembly mechanism is required.

## License

This project is released under the MIT License.
