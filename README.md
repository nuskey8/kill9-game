# kill9

A mini-game playable in the terminal, inspired by the kill command.

https://github.com/nuskey8/kill9-game/blob/main/assets/gameplay.mp4

## Install

### Releases

Get the latest binaries for Windows/macOS/Linux from the latest [releases](https://github.com/nuskey8/kill9-game/releases/latest).

### Homebrew

```bash
$ brew install nuskey8/tap/kill9-game
```

## How to Play

- Kill the processes that appear one after another using `kill <PID>`.
- Processes marked `[Not Responding]` cannot be deleted without specifying the `-9` option. These processes place a heavy load on RAM, so prioritize killing them.
- You can use the `killall` command up to three times. This will kill all processes at once.
- The game ends when the RAM reaches 100%.