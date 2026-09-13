# CLAUDE.md — Freezy Arena (Cheesy Arena fork)

Go Field Management System (FMS) for FRC-style events. Fork of
[Team254/cheesy-arena](https://github.com/Team254/cheesy-arena) (Go module path is still
`github.com/Team254/cheesy-arena`; do not rename it). Local-only web server: all human interaction is
through a browser, state is pushed to clients over WebSockets, data lives in a BoltDB file.

## What this system actually controls

### 1. Team networking (Cisco Catalyst switch)
[network/switch.go](network/switch.go) configures a Catalyst 3500-series switch over **Telnet (port 23)**
so this computer can reach up to 6 driver station PCs, each isolated on its own VLAN:

| Station | VLAN | Team subnet | Gateway / switch SVI |
|---|---|---|---|
| R1/R2/R3 | 10 / 20 / 30 | `10.<team/100>.<team%100>.0/24` | `.4` |
| B1/B2/B3 | 40 / 50 / 60 | same scheme | `.4` |

- On each match load, old team VLANs are torn down and re-added with fresh DHCP pools (`dhcp10`…`dhcp60`).
- The FMS host must be **10.0.100.5** on a real field — the FRC Driver Station hardcodes that address.
  Run with `-dev` to bind DS listeners to all local IPs instead (`network.DevMode`).
- [switch_config.txt](switch_config.txt) is the baseline switch config to load before pointing the FMS at
  the switch. Default switch password in that file is `1234Five`.
- [network/access_point.go](network/access_point.go) handles team wifi (Vivid-Hosting style AP) — separate
  from the wired VLAN work.

### 2. Per-field SCC switch (Freezy addition)
A "field" is red + blue alliance. Each field has an **SCC switch** that the local Windows driver stations
plug into. [network/sccswitch.go](network/sccswitch.go) drives it over **SSH (port 22)** with raw command
lists — no VLAN logic, just enable/disable of team ethernet ports:

- Settings: `SCCManagementEnabled`, `RedSCCAddress`, `BlueSCCAddress`, `SCCUsername`, `SCCPassword`,
  `SCCUpCommands`, `SCCDownCommands` (the up/down commands are newline-separated free text from the
  settings page, split in [field/arena.go:209](field/arena.go#L209)).
- `arena.setSCCEthernetEnabled(bool)` ([field/arena.go:1079](field/arena.go#L1079)) is called from the match
  state machine — ports go down/up around match transitions. Red and blue are configured in parallel
  goroutines; failures are logged, not fatal.
- `Status` on both `Switch` and `SCCSwitch` is a plain string: `UNKNOWN` / `CONFIGURING` / `ACTIVE` /
  `DISABLED` / `ERROR`, surfaced to the UI.

### 3. ESP32 field devices (the "Alternate IO" path)
Instead of (or alongside) the Allen-Bradley PLC, this fork supports **ESP32 HTTP devices**: LED light
stacks and buttons — driver station E-Stops and A-Stops, a field E-Stop, and a Start button. There are
three logical devices, each just an IP address in settings:

- `ScoreTableEstopAddress`, `RedAllianceStationEstopAddress`, `BlueAllianceStationEstopAddress`, gated by
  `AlternateIOEnabled`.
- [plc/esp32IO.go](plc/esp32IO.go) (`plc.Esp32` / `Esp32IO`) only does **health checking** — a 1 s loop that
  TCP-dials port 80 on each configured IP and tracks `*Healthy` flags, plus calls `Plc.ResetMatchReset()`.
  An empty IP means "not enabled". Health is published via
  [field/arena_notifiers.go:133](field/arena_notifiers.go#L133).
- The real data flow is in [web/alternateIO.go](web/alternateIO.go) — the ESP32s are HTTP clients that
  **poll GETs and POST their inputs**:

| Route | Direction | Purpose |
|---|---|---|
| `POST /api/freezy/eStopState` | ESP32 → FMS | `[{channel,state}]`, written into PLC input array via `SetAlternateIOStopState` |
| `POST /api/freezy/startMatch` | ESP32 → FMS | Start button |
| `GET /api/freezy/field_stack_light` | FMS → ESP32 | red/blue/orange/green field stack light |
| `GET /api/freezy/team_stack_light` | FMS → ESP32 | per-station 2-layer stack light (color + blink) |
| `GET /api/freezy/alternateIO/PLC_Coils` | FMS → ESP32 | all coils as name→bool |
| `POST /api/freezy/register_values` | ESP32 → FMS | set PLC registers |
| `GET /api/plc/websocket` | either | live PLC IO + LED modes; accepts `setPLCRegister`/`setRegisters`/`setInput` messages |

Key gotcha: **E-Stop/A-Stop inputs are active-low.** `true` = not pressed. See
`ResetEstops()` ([plc/plc.go:573](plc/plc.go#L573)) which sets every stop input to `true`. `channel` in the
POST payload is an index into the `input` enum in [plc/plc.go](plc/plc.go) — if you reorder those constants
you break every deployed ESP32.

### 4. PLC (upstream path, still primary abstraction)
[plc/plc.go](plc/plc.go) talks Modbus TCP (port 502) to an Allen-Bradley PLC; everything else in the
codebase depends on the `plc.Plc` **interface**, not the hardware. `AlternateIOEnabled` lets the ESP32
HTTP layer write into the same in-memory input/coil arrays, so the arena state machine is unaware of which
backend is in use. Freezy-only interface methods are grouped under a `// Freezy Arena` comment near the
bottom of the interface — keep that grouping.

`arena.Plc.IsEnabled()` is false when `PlcAddress` is blank; several checks are of the form
`if !arena.Plc.IsEnabled() && !arena.EventSettings.AlternateIOEnabled`
([field/arena.go:1304](field/arena.go#L1304)). Any new PLC-gated behavior needs the same both-paths check.

### 5. Hub / goal LEDs
[led/](led/) is **E1.31 sACN (DMX over Ethernet), UDP 5568** — not the ESP32 stacks and not Advatek.
`LedControllerAddress` in settings. 16 fixtures (red/blue × 4 sides × top/bot), 8 pixels each, mapped in
[led/fixture.go](led/fixture.go). Modes live in [led/mode.go](led/mode.go) (includes the
`Side1TestMode`…`Side4TestMode` test modes); match-state-driven mode selection is in
[field/arena_leds.go](field/arena_leds.go), pushed to UI via `LedChangeNotifier`.

## Layout

- [main.go](main.go) — flags, opens `./event.db`, `field.NewArena`, serves HTTP on **8080**, then
  `arena.Run()` on the main thread.
- [field/](field/) — `Arena` god-object + 10 ms state machine loop. `MatchState`: `PreMatch`, `StartMatch`,
  `AutoPeriod`, `PausePeriod`, `TeleopPeriod`, `PostMatch`, `TimeoutActive`, `PostTimeout`. Also driver
  station UDP/TCP protocol ([field/driver_station_connection.go](field/driver_station_connection.go)),
  realtime score, team signs, per-match team logs, and all notifiers
  ([field/arena_notifiers.go](field/arena_notifiers.go)).
- [game/](game/) — current-season scoring rules (`hub.go`, `score.go`, `ranking_fields.go`, match timing,
  sounds). Season-specific; the most churn on merges.
- [model/](model/) — BoltDB tables and `EventSettings` (every hardware address/toggle above).
- [network/](network/), [plc/](plc/), [led/](led/) — hardware, as above.
- [web/](web/) — one file per page/panel + `web.go` route table; [templates/](templates/) HTML,
  [static/](static/) JS/CSS/img/sounds.
- [websocket/](websocket/) — `Notifier` fan-out; clients subscribe with `ws.HandleNotifiers(...)`.
- [playoff/](playoff/), [tournament/](tournament/), [schedules/](schedules/) — brackets (incl. Freezy's
  4–8 team double elim), rankings, pre-generated schedules.
- [partner/](partner/) — TBA, Nexus, Blackmagic, Companion integrations.
- [Python Test File/](Python%20Test%20File/) — hand-run scripts that simulate PLC/FTA-ready traffic
  (`pip install readchar`). Not part of the build or CI.

## Commands

```bash
go build              # produces ./cheesy-arena in repo root
./cheesy-arena        # then open http://localhost:8080
./cheesy-arena -dev   # no real driver stations / not on 10.0.100.5
go test ./...         # full suite; green as of this file's writing
go test ./field -run TestName
go fmt ./...          # required before committing
go generate ./...     # regenerate *_string.go after editing enum constants
```

Enum-string files that `go generate` owns: [plc/input_string.go](plc/input_string.go),
[plc/coil_string.go](plc/coil_string.go), [plc/register_string.go](plc/register_string.go),
[plc/armorblock_string.go](plc/armorblock_string.go), [model/matchtype_string.go](model/matchtype_string.go).

`event.db`, `build/`, and the `cheesy-arena` binary are gitignored — never commit them.

## Conventions (see also [AGENTS.md](AGENTS.md))

- Standard Go style, tabs, `gofmt`. **Imports: one alphabetical block, no grouping, no stdlib/third-party
  split, no goimports.** Much of the Freezy-added code (e.g. `plc/esp32IO.go`, `web/alternateIO.go`) does
  not follow this and uses spaces/odd alignment — match the file you're in unless you're cleaning it up
  deliberately.
- Fork-added code is marked with a `// Freezy Arena` comment before the block (route table, PLC interface
  methods, settings fields). Keep new fork-only additions in those marked sections — it is what makes
  upstream merges tractable.
- Tests are `*_test.go` co-located with the package; prefer table-driven. `field/test_helpers.go`,
  `model/test_helpers.go`, `game/test_helpers.go`, `plc/fake_modbus_client_test.go`, and
  `field/fake_plc_test.go` already exist — use them rather than new fakes.
- Commit messages: short imperative sentence, optional `(#123)` suffix. PRs: summary, exact test commands
  run, screenshots for `web/`/`templates/`/`static/` changes.

## Merging features from another branch

Remotes: `origin` = `Freezy-Arena/freezy-arena` (this fork), `old-fork` =
`Byron-MN-FRC/fms-freesy-arena`. Upstream Team254 is **not** configured as a remote — add it explicitly if
you need upstream commits. Feature branches on origin include `FREEZY_ARENA_2026`, `DMX_Websocket`,
`FTA-Team-Score-DIsplay`, `Repair-Timing-for-Game-Like-Exspirence`, `external-DB`,
`2026_week_0_NMRC(_patches)`. Recent history shows both PR merges and cherry-pick batches from upstream.

High-conflict areas to expect, in rough order:

1. `model/event_settings.go` + [web/setup_settings.go](web/setup_settings.go) +
   [templates/setup_settings.html](templates/setup_settings.html) — every hardware feature adds a field in
   all three. Field order matters for readability only, but forgetting the form handler or the template
   silently drops the setting.
2. `plc/plc.go` enums (`input`, `coil`, `register`) — reordering renumbers wire indices used by ESP32s and
   the PLC program; append, don't insert. Regenerate the `*_string.go` files after.
3. `plc.Plc` interface — upstream adds methods; fakes in `field/fake_plc_test.go` must implement them.
4. [web/web.go](web/web.go) route table — Freezy routes are all under `/api/freezy`, `/panel/freezy`,
   `/help/freezy`, in a marked block at the end.
5. `field/arena.go` (`Arena` struct + `LoadSettings` + state machine) and `field/arena_notifiers.go`.
6. `game/` — if the merge crosses a season, scoring/ranking conflicts are effectively rewrites, not merges.

After any merge: `go build && go test ./... && go fmt ./...`, then exercise the settings page and the
field monitor in a browser, since much of the fork's wiring is template/JS-side and untested.
