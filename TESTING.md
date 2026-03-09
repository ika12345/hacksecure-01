# Testing the Lowkey Signaling Handshake

This guide explains how to simulate a WebRTC signaling handshake between two peers using the `lowkey` CLI and gRPC server.

## Prerequisites

- **Redis**: Ensure Redis is running locally.
  ```bash
  docker run -p 6379:6379 redis
  ```

## Step 1: Start the Signaling Server

In your first terminal, build and run the gRPC server:

```bash
go build -o signaling-server ./server
./signaling-server
```

The server will listen on `[::]:50051`.

## Step 2: Start the Listening Peer (Terminal A)

In a second terminal, build the CLI and start the `listen` command. This peer will register a unique UUID and wait for signals.

```bash
go build -o lowkey .
./lowkey listen
```

**Note**: Copy the **My UUID** value printed in this terminal (e.g., `550e8400-e29b-41d4-a716-446655440000`).

## Step 3: Send a Signal (Terminal B)

In a third terminal, use the `send` command to fire a mock SDP Offer to the listener's UUID.

```bash
./lowkey send --target <SENDER_UUID_FROM_STEP_2>
```

## Expected Results

1. **Terminal B (Sender)**: Should print `SDP Offer sent successfully!`.
2. **Terminal A (Listener)**: Should display the `[SDP Received]` block containing the mock SDP data.
3. **Server Logs**: Should show `Client connected` and routing logs if implemented.

## Cleaning Up

Use `Ctrl+C` to stop the server and the listener.

---

## Quick Demo: Live Chat Between Two Peers (No Android Studio Needed)

You can see **real text messages transfer between two simulated devices** entirely from the command line — no Android Studio, no mobile device required.

This uses the same signaling infrastructure that the mobile app relies on, letting you observe messages flowing between peers in real time.

### Step 1: Start Infrastructure (same as above)

```bash
# Terminal 0 – Redis
docker run -p 6379:6379 redis

# Terminal 1 – Signaling server
go build -o signaling-server ./server
./signaling-server
```

### Step 2: Build the CLI

```bash
go build -o lowkey .
```

### Step 3: Start Peer A (listener role)

```bash
# Terminal A
./lowkey chat
```

Sample output:
```
My UUID: 550e8400-e29b-41d4-a716-446655440000
Waiting for a peer to connect... (share your UUID above)
```

Copy the UUID printed by Peer A.

### Step 4: Start Peer B (initiator role)

```bash
# Terminal B – replace <UUID> with Peer A's UUID
./lowkey chat --target 550e8400-e29b-41d4-a716-446655440000
```

Sample output:
```
My UUID: 7f3e1a2b-c4d5-6789-abcd-ef0123456789
Connected to peer 550e8400-e29b-41d4-a716-446655440000
Type messages and press Enter to send. Press Ctrl+C to exit.
>
```

### Step 5: Exchange Messages

Both terminals are now connected. Type a message in either terminal and press **Enter** — it appears instantly in the other terminal.

**Terminal B types:**
```
> Hello from Peer B!
```

**Terminal A receives:**
```
Peer connected: 7f3e1a2b-c4d5-6789-abcd-ef0123456789
Type messages and press Enter to send. Press Ctrl+C to exit.
> 
[peer] Hello from Peer B!
> Hi back from Peer A!
```

**Terminal B receives:**
```
> [peer] Hi back from Peer A!
```

Messages are routed through the gRPC signaling server and Redis — the same path used by the React Native mobile app.

### What Requires Android Studio?

The CLI chat demo covers the **signaling layer** (the part that sets up a WebRTC connection). The full mobile app additionally provides:

- A native WebRTC `RTCPeerConnection` with audio/video/data channels
- End-to-end encrypted data channel messages
- Persistent local storage via WatermelonDB

To run the mobile app, follow the setup guide in [`frontend/README.md`](frontend/README.md).

