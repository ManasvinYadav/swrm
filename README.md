```
███████╗██╗    ██╗██████╗ ███╗   ███╗
██╔════╝██║    ██║██╔══██╗████╗ ████║
███████╗██║ █╗ ██║██████╔╝██╔████╔██║
╚════██║██║███╗██║██╔══██╗██║╚██╔╝██║
███████║╚███╔███╔╝██║  ██║██║ ╚═╝ ██║
╚══════╝ ╚══╝╚══╝ ╚═╝  ╚═╝╚═╝     ╚═╝
```

<p align="center"><b>Local-first BitTorrent, in your terminal.</b></p>

<p align="center">
  <a href="https://www.npmjs.com/package/swrm"><img alt="npm" src="https://img.shields.io/npm/v/swrm.svg"></a>
  <a href="https://github.com/ManasvinYadav/swrm/releases"><img alt="release" src="https://img.shields.io/github/v/release/ManasvinYadav/swrm"></a>
  <a href="./LICENSE"><img alt="license" src="https://img.shields.io/badge/license-MIT-informational.svg"></a>
  <img alt="platforms" src="https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey">
</p>

`swrm` is a keyboard-driven BitTorrent client that lives entirely in your terminal. There
is no built-in search or indexer — you bring a magnet URI, an infohash, or a local
`.torrent` file, and `swrm` does the rest: metadata resolution, piece scheduling, a live
swarm dashboard, and one-key streaming into `mpv`/`vlc`/`iina` while the torrent is still
downloading.

## Features

- **Local-first, always.** Paste a `magnet:?xt=urn:btih:...` URI, a bare 40-char hex /
  32-char Base32 infohash, or browse to a `.torrent` file on disk. No bundled indexers,
  no telemetry, no network calls beyond DHT/trackers/peers.
- **VPN kill-switch.** Point `swrm` at a specific network interface (e.g. `tun0`, `wg0`)
  and every socket — peers, trackers, DHT — is bound to it. If the interface drops,
  `swrm` halts all transfer activity immediately rather than leaking your real IP.
- **Multi-torrent, one dashboard.** Add several torrents; switch which one is
  "highlighted" with the arrow keys and a slim switcher strip appears automatically.
- **Real pause/resume**, per torrent — not a rate-limit hack.
- **Stream while downloading.** Sequential piece prioritization keeps a rolling playback
  buffer ahead of your player, with stall detection and priority escalation for anything
  that falls behind.
- **A dashboard that stays out of your way.** One persistent screen — magnet input, file
  browser, transfer inspector — with a 3-stop keyboard focus cycle instead of buried
  menus.

## Install

**npm (recommended — installs a prebuilt binary, no Go toolchain required):**

```sh
npm install -g swrm
swrm
```

**Or run it on demand without installing anything:**

```sh
npx swrm
```

**From source** (requires Go 1.27+):

```sh
git clone https://github.com/ManasvinYadav/swrm.git
cd swrm
go build -o swrm ./cmd/swrm
./swrm
```

## Quick look

```
┌────────────────────────────────────────────────────────────────────┐
│ ⌘ Enter   magnet URI or hash... magnet:?xt=urn:btih:...             │
├───────────────────────────────┬──────────────────────────────────--┤
│ ╭─ Inbuilt .torrent browser ──╮│╭─ Active inspector & transfer ───╮ │
│ │ 📁 Downloads                ││ ubuntu-24.04.1-live-server.iso   │ │
│ │ 📁 Documents                ││ ████████████████░░░░░░░░  64%    │ │
│ │ 📁 Movies                   ││ ↑ DL 12.4 MB/s   ↓ UL 1.1 MB/s    │ │
│ │ 📄 arch-linux.torrent       ││ Swarm: ▰▰▰▱▰▰▱▱▰▰▰▰▱▰▰▱▰▰▰▱       │ │
│ │ 📄 debian-net-inst.torrent  ││ ● 122.100.133.4   qBittorrent     │ │
│ ╰──────────────────────────────╯╰───────────────────────────────╯ │
├────────────────────────────────────────────────────────────────────┤
│ [1] Search  [2] Transfers  [3] Swarm  [4] Streaming   ● VPN: tun0   │
│           esc back · tab switch · ↵ submit · space pause · q quit  │
└────────────────────────────────────────────────────────────────────┘
```

## Keybindings

| Key                | Action                                                          |
| ------------------ | ---------------------------------------------------------------- |
| `Tab` / `Shift+Tab` | Cycle keyboard focus: magnet input → file browser → inspector |
| `1`                 | Jump focus to the magnet input                                 |
| `2` / `3`           | Emphasize the inspector's transfer / swarm section              |
| `4`                 | Stream the highlighted torrent (launches `mpv`/`vlc`/`iina`)     |
| `←` / `→`           | Switch the highlighted torrent (when the inspector has focus)   |
| `Space`             | Pause / resume the highlighted torrent                          |
| `Enter`             | Add a magnet (header) or open/select a file (browser)           |
| `d`                 | Toggle the diagnostics panel                                     |
| `Esc`               | Close a modal / go back                                          |
| `q` / `Ctrl+C`      | Quit                                                             |

## Configuration

`swrm` reads `~/.config/swrm/config.yaml` if it exists; every field is optional. See
[`config.example.yaml`](./config.example.yaml):

```yaml
interface: "wg0"          # network interface to bind to; "" = standard system routing
dht: true
listen_port: 6881
download_dir: "~/Downloads/swrm"
media_player: "auto"      # "auto", or one of mpv / vlc / iina
post_download_cmd: ""     # shell command to run after a transfer completes
download_limit: 0         # bytes/sec, 0 = unlimited
upload_limit: 0
```

You can also point at an interface without a config file:

```sh
swrm -interface tun0
```

If `interface` is set, `swrm` refuses to start unless that interface exists and is up,
and every packet is bound to it for the lifetime of the process.

## How it works

```
cmd/swrm/          entrypoint — config, VPN manager, engine, and the Bubble Tea program
internal/config/   YAML + flag config loading
internal/engine/   BitTorrent client wrapper, multi-torrent registry, VPN kill-switch,
                    sequential piece picker, per-torrent snapshots
internal/server/   local HTTP range server + media player launcher, for streaming
internal/ui/       the Bubble Tea dashboard (header, file browser, inspector, modals)
```

Built on [`anacrolix/torrent`](https://github.com/anacrolix/torrent) and
[Charm](https://charm.sh)'s `bubbletea` / `bubbles` / `lipgloss`.

## Development

```sh
go build ./...
go vet ./...
go test ./...
```

## Releasing (maintainers)

Pushing a tag matching `v*` (e.g. `v0.1.0`) triggers
[`.github/workflows/release.yml`](./.github/workflows/release.yml), which:

1. Cross-compiles `swrm` for `darwin/arm64`, `darwin/amd64`, `linux/amd64`,
   `linux/arm64`, and `windows/amd64` with stripped symbols (`-ldflags="-s -w"`).
2. Publishes a GitHub Release with all five binaries attached.
3. Syncs `npm/package.json`'s version to the tag and runs `npm publish --access public`
   from `npm/`, using an `NPM_TOKEN` secret.

The `npm/` package itself ships no binary — its `postinstall` script downloads the
matching release asset for the current platform/arch from GitHub Releases at install
time, which is how `npm install -g swrm` and `npx swrm` stay lightweight.

Before the first release, add an npm automation token as a repository secret named
`NPM_TOKEN` (Settings → Secrets and variables → Actions).

## Security

`swrm` does not bundle or recommend any indexer, tracker, or search source — what you
download is entirely up to you, and it's your responsibility to ensure you have the
right to do so. The VPN kill-switch is a best-effort leak-prevention mechanism, not a
guarantee; verify your own network setup if anonymity matters to you.

## License

[MIT](./LICENSE)
