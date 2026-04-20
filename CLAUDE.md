# KVM Project

OpenKVM-based KVM-over-IP project. Controls a remote computer via V4L HDMI capture + ESP32-S3 USB HID + noVNC browser client.

## Project Structure

```
kvm/                          # Project root
├── openkvm/                  # OpenKVM backend (Go, RFC 6143 VNC)
│   ├── km/esp32s3-arduino/  # ESP32-S3 HID firmware (Arduino)
│   │   └── main/main.ino     # Main ESP32 firmware
│   ├── kvm/                 # VNC server implementation
│   ├── ui/                  # Web UI assets
│   ├── kvm.new.toml         # Configuration template
│   └── main.go              # Entry point
├── noVNC/                   # Browser-based VNC client
└── CLAUDE.md                # This file
```

## Hardware Setup

| Device | Connection | Path |
|--------|-----------|------|
| HDMI Capture (V4L) | USB | `/dev/video0` |
| ESP32-S3 (HID) | Serial UART | `/dev/ttyACM0` or `/dev/ttyUSB0` |

## Configuration

1. Copy and edit config:
   ```bash
   cp openkvm/kvm.new.toml openkvm/kvm.toml
   vim openkvm/kvm.toml
   ```

2. Key settings in `kvm.toml`:
   - `video.src` — V4L capture command (e.g., `v4l2-ctl --device=/dev/video0 ...`)
   - `keyboard.src` / `mouse.src` — ESP32 serial port (e.g., `/dev/ttyACM0`)
   - `novnc.path` — Path to noVNC folder (e.g., `../noVNC`)

3. VNC credentials (default): `openkvm` / `passwd12`

## Running

```bash
cd openkvm
go run . -c kvm.toml
# Access at http://<host>:8080/vnc.html
```

## ESP32-S3 Firmware

### Flashing (Recommended)

Use the automated flash script (installs Arduino CLI if needed):
```bash
cd openkvm
./flash-esp32.sh              # Auto-detect serial port
./flash-esp32.sh /dev/ttyACM0 # Specify port manually
```

### Manual Flashing

If Arduino CLI is already installed:
```bash
cd openkvm

# Compile
arduino-cli compile -b esp32:esp32:esp32s3 km/esp32s3-arduino/main/

# Upload (specify your serial port)
arduino-cli upload -b esp32:esp32:esp32s3 -p /dev/ttyACM0 km/esp32s3-arduino/main/
```

### Monitoring Serial Output

After flashing, connect to see debug output:
```bash
screen /dev/ttyACM0 921600
```
Press `Ctrl+A` then `k` to exit screen.

### Finding the Serial Port

If ESP32 doesn't appear at `/dev/ttyACM0`, check:
```bash
ls -la /dev/ttyACM* /dev/ttyUSB*  # List all serial devices
dmesg | grep tty                  # Recent device connections
```

## V4L Commands

```bash
# List devices
v4l2-ctl --list-devices

# List formats
v4l2-ctl --list-formats -d 0

# Capture single frame
v4l2-ctl --device=/dev/video0 --stream-mmap --stream-count=1 --stream-to=frame.jpg --set-fmt-video="width=1280,height=720,pixelformat=MJPG"

# Reset stuck USB device
usbreset <device_id>
```

## Remote Host (192.168.4.3)

- SSH: `sam@192.168.4.3`
- V4L device: Check with `v4l2-ctl --list-devices`
- ESP32 serial: Check with `dmesg | grep tty`
