# idle-timeout

Kill a process if no stdout/stderr output for a specified duration. Unlike GNU `timeout` which uses absolute time limits, `idle-timeout` monitors output activity and only kills the process if it becomes idle.

Perfect for wrapping CLI tools that occasionally hang or stall.

## Installation

```bash
go install github.com/gavlooth/idle-timeout@latest
```

Or build from source:

```bash
git clone https://github.com/gavlooth/idle-timeout.git
cd idle-timeout
go build -o idle-timeout .
sudo cp idle-timeout /usr/local/bin/
```

## Usage

```bash
idle-timeout <duration> <command> [args...]
```

Duration can be:
- A number (interpreted as seconds): `30`, `300`
- A Go duration string: `30s`, `5m`, `1h30m`

## Examples

```bash
# Kill if no output for 30 seconds
idle-timeout 30 curl -s https://example.com

# Kill if no output for 5 minutes
idle-timeout 5m ./long-running-script.sh

# Use with GNU parallel for batch processing
seq 10 | parallel -j1 'idle-timeout 300 mycommand "$(cat ~/prompt)"'
```

## Features

- **Inactivity-based timeout**: Only kills when there's no output, not after a fixed time
- **PTY support**: Preserves colors, progress bars, and interactive output
- **Exit code 124**: Returns 124 when killed due to timeout (same as GNU timeout)
- **Signal forwarding**: Ctrl+C and other signals are forwarded to the child process
- **Terminal resize**: Handles terminal resize events properly

## Exit Codes

- `124`: Process was killed due to inactivity timeout
- Other: Exit code of the wrapped command

## Why?

GNU `timeout` and `timelimit` use absolute time limits. If your command normally takes 10 minutes but occasionally hangs, you'd need to set a timeout of 15+ minutes to be safe.

`idle-timeout` solves this by monitoring output. A command can run for hours as long as it keeps producing output. But if it goes silent for the specified duration, it gets killed.

This is especially useful for:
- CI/CD pipelines with flaky commands
- Batch processing with CLI tools that occasionally hang
- Running LLM CLI tools that might stall

## License

MIT
