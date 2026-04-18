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
| janus | 8188 (WS), 8088 (HTTP), 7088 (Admin) | Janus WebRTC gateway |
| coturn | 3478 (STUN/TURN) | NAT traversal server |
| janus-web | 80 | Static file server for web client |

## Quick Start

```bash
# Start all services
docker compose up -d

# Check status
docker compose ps

# View logs
docker compose logs -f janus
```

## Accessing

- **Web Client**: http://localhost:8180/janus.html or http://localhost:8180/pikvm.html
- **Janus Admin**: http://localhost:7088/admin/
- **WebSocket**: ws://localhost:8188/janus

## Configuration

### Janus Room

The default room ID is `1234`. To join with a browser:

```
http://localhost:8180/janus.html?room=1234
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
janus_url = "ws://localhost:8188/janus"
room = 1234
```

## Production Deployment

### Firewall Ports

Open these UDP/TCP ports on your firewall:

```bash
# Janus WebSocket
TCP 8188

# Janus HTTP (admin)
TCP 8088

# Janus Admin
TCP 7088

# TURN STUN/TURN
UDP 3478

# WebRTC media (RTP/RTCP)
UDP 10000-10099
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
   curl -s http://localhost:7088/admin/info
   ```

2. Check WebSocket:
   ```bash
   wscat -c ws://localhost:8188/janus
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
