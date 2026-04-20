# Janus Gateway Deployment

This directory contains Docker Compose configuration for deploying the Janus WebRTC gateway, which enables low-latency KVM streaming.

## Architecture

```
┌─────────────┐     ┌─────────────────┐     ┌─────────────────────┐
│   Browser   │────▶│  Janus Gateway   │────▶│  OpenKVM Backend    │
│  (WebRTC)  │◀────│  (WebSocket)    │◀────│  (H.264 Video)      │
└─────────────┘     └─────────────────┘     └─────────────────────┘
                           │
                           ▼
                    ┌─────────────────┐
                    │  Coturn (STUN)  │
                    │  (NAT Traversal)│
                    └─────────────────┘
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| janus | 27100 (WS), 28088 (HTTP), 27088 (Admin) | Janus WebRTC gateway |
| coturn | 3478 (STUN/TURN) | NAT traversal server |
| janus-web | 8180 | Static file server for web client |

## Quick Start

```bash
# Start all services
docker compose up -d

# Check status
docker compose ps

# View logs
docker compose logs -f janus
```

**Note:** On Mac with Colima, you may need to use different ports due to SSH port interception.
If WebSocket connections fail, check port availability:
```bash
# Find free ports
for port in 27100 28088 27088; do
  lsof -i :$port 2>/dev/null | grep LISTEN || echo "Port $port is free"
done
```

## Accessing

- **Web Client (Subscriber)**: http://localhost:8180/ - Connects as publisher to receive video
- **Test Publisher**: http://localhost:8180/test-publisher.html - Streams sample.webm for testing
- **Janus Admin**: http://localhost:7088/admin/
- **WebSocket**: ws://localhost:27100/janus

### URL Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `room` | Video room ID | `?room=1234` |
| `mode` | Connection mode: `auto` (default), `publisher`, `subscriber` | `?mode=subscriber` |
| `feed` | Publisher feed ID to subscribe to (required for subscriber mode) | `?feed=123456789` |

### Testing Flow

1. Open http://localhost:8180/test-publisher.html?room=1234 in one browser tab
2. Click "Start Publishing" - the page shows a Feed ID
3. Open http://localhost:8180/?room=1234&mode=subscriber&feed=<FEED_ID> in another tab
4. The second tab should receive and display the video stream

## Configuration

### Janus Room

The default room ID is `1234`. To subscribe to a video stream:

```
http://localhost:8180/?room=1234
```

### TURN Credentials

Default credentials (change in production):

| Username | Password |
|----------|----------|
| openkvm | openkvm123 |

### OpenKVM Configuration

Enable WebRTC in your `kvm.toml`:

```toml
[webrtc]
enabled = true
janus_url = "ws://localhost:27100/janus"
room = 1234
```

### Testing with sample.webm

A test video source is available at `sample.webm` in the nginx document root. Use the test publisher to stream it:

1. Open http://localhost:8180/test-publisher.html?room=1234
2. Click "Start Publishing"
3. Note the Feed ID displayed on the page
4. Subscribe using `?room=1234&mode=subscriber&feed=<FEED_ID>`

## Production Deployment

### Firewall Ports

Open these UDP/TCP ports on your firewall:

```bash
# Janus WebSocket (external)
TCP 27100

# Janus HTTP (admin, external)
TCP 28088

# Janus Admin (internal)
TCP 27088

# TURN STUN/TURN
UDP 3478

# WebRTC media (RTP/RTCP)
UDP 27101-27200 (maps to 10000-10099 inside container)
```

### Security

1. **Change TURN credentials** in `docker-compose.yml`:
   ```yaml
   CREDENTIALS: "your-user:your-password"
   ```

2. **Enable HTTPS** with a reverse proxy (nginx, traefik) for production

3. **Secure the Janus Admin interface** with firewall rules

### Janus Log Levels

To debug WebRTC issues:

```bash
# View Janus logs with WebRTC trace
docker compose logs -f janus 2>&1 | grep -i webrtc
```

## Troubleshooting

### Connection Fails

1. Check Janus is running:
   ```bash
   curl -s http://localhost:28088/janus/info
   ```

2. Check WebSocket:
   ```bash
   wscat -c ws://localhost:27100/janus -e janus-protocol
   ```

### Video Not Playing

1. Verify H.264 encoding is working
2. Check browser console for SDP/ICE errors
3. Ensure firewall allows UDP 10000-10099

### NAT Traversal Issues

If clients are behind strict NAT:

1. Ensure Coturn is running:
   ```bash
   docker compose logs coturn
   ```

2. Check STUN works:
   ```bash
   docker exec coturn turnutils_stunclient -v stun.l.google.com:19302
   ```

## PiKVM Reference

This setup is inspired by [PiKVM](https://github.com/pikvm/pikvm) which uses Janus for WebRTC streaming.
