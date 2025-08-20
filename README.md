## Disguise: A Go-based Library for Network Traffic Obfuscation

### Introduction

Disguise is a network traffic obfuscation protocol library written in Go. Its goal is to evade Deep Packet Inspection (DPI) and traffic analysis by disguising communication patterns. By combining techniques like **cover traffic**, **content-aware padding**, and **dynamic traffic shaping**, it hides your real data within seemingly natural background traffic, thereby enhancing network anonymity and censorship circumvention.

This library provides a set of core components that can be easily integrated into any application requiring traffic obfuscation, such as VPNs, proxies, or instant messaging tools.

### Core Concepts

The library is built on four main components that work together to achieve traffic obfuscation:

  - **Profile**: Defines various traffic pattern parameters, such as those for web browsing, video streaming, and file downloading. Each profile contains unique cell size distributions, latency jitters, and probing intervals to accurately mimic specific behaviors.
  - **Framer**: Is responsible for segmenting application-layer data into protocol cells and creating dummy cells for cover traffic. It implements content-aware padding, which generates realistic-looking padding data, such as fake HTTP/2 headers or Base64 encoded data, based on the current traffic type.
  - **Scheduler**: Uses a priority queue to manage the order of packet transmission. It ensures that real data is prioritized while seamlessly inserting cover traffic into the data stream to maintain a stable transmission rate and pattern.
  - **Manager**: The core orchestrator of the entire protocol. It connects the Framer and Scheduler and includes a dynamic traffic profiler. This profiler monitors the traffic load in real time and automatically switches to the most suitable profile as needed, enabling an adaptive obfuscation strategy.

### Installation

Make sure you have a Go environment installed. Then, use the following command to install the library:

```bash
go get github.com/uDisguise/disguise
```

### Usage Example

The following code demonstrates how to use the Disguise library for data transmission within your application.

```go
package main

import (
	"fmt"
	"github.com/uDisguise/disguise/disguise"
)

func main() {
	// 1. Initialize the Disguise manager
	//    This will start the background dynamic traffic profiling and cover traffic loops.
	manager := disguise.NewManager()
	
	// 2. Simulate application-layer data to be sent
	applicationData := []byte("Hello, this is a secret message that needs to be disguised from traffic analysis!")
	
	fmt.Printf("Original application data length: %d bytes\n", len(applicationData))

	// 3. Hand over the application data to the manager for obfuscation
	err := manager.QueueApplicationData(applicationData)
	if err != nil {
		fmt.Printf("Failed to queue data: %v\n", err)
		return
	}

	// 4. Get the obfuscated packets from the manager and simulate network transmission
	var totalOutboundBytes int
	for {
		// Get the next available packet from the manager
		packet, err := manager.GetOutboundTraffic()
		if err == disguise.ErrNoOutboundTraffic {
			// No more data to send, exit the loop
			break
		}
		if err != nil {
			fmt.Printf("Failed to get outbound traffic: %v\n", err)
			return
		}

		totalOutboundBytes += len(packet)
		
		// Here, you would send the `packet` to your network connection
		// Simulating sending to the network...
		// fmt.Printf("Sending an obfuscated packet, length: %d bytes\n", len(packet))

		// Simulate receiving the packet back and processing it with the manager
		err = manager.ProcessInboundTraffic(packet)
		if err != nil {
			fmt.Printf("Failed to process inbound traffic: %v\n", err)
			return
		}
	}
	
	// 5. Read the de-obfuscated application data from the manager
	reassembledData, err := manager.ReadApplicationData()
	if err != nil {
		fmt.Printf("Failed to read application data: %v\n", err)
		return
	}

	fmt.Printf("Total bytes sent after obfuscation: %d bytes\n", totalOutboundBytes)
	fmt.Printf("Received de-obfuscated data: %s\n", string(reassembledData))
}
```

### Customization

You can adjust the behavior of Disguise in the following ways:

  - **Dynamically Change Profiles**: You can manually switch traffic modes at runtime by calling the `SetProfile` method to adapt to different application scenarios.

<!-- end list -->

```go
// Switch to video streaming mode
manager.SetProfile(profile.GetProfile(profile.VideoStreaming))
```

  - **Create Custom Profiles**: You can modify the `disguise/profile/profile.go` file to add or adjust parameters, creating unique traffic patterns that suit your needs.

### Limitations and Future Work

  - **Simplified Machine Learning Model**: The current dynamic profiler is a simplified rule engine. Future versions could integrate more complex statistical models or actual machine learning algorithms for more precise traffic pattern recognition and switching.
  - **Single-Stream Reassembler**: The current reassembler only supports processing a single data stream. For more complex applications, it needs to be refactored to support parallel reassembly of multiple streams.
  - **Network Integration**: This library is a protocol core; it needs to be integrated with a complete network application (like a TLS `net.Conn`) to actually send and receive obfuscated data.

### License

This library is released under the MIT License.
