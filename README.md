# go-cipher

Terminal UI (Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea)) for encrypting
and decrypting text and files. Implements **GOST 28147-89** and **STB 34.101.31 (BelT)**.

![go-cipher TUI](Reports/Lab1/img/tui-cipher.png)

## Features

- Two algorithms: GOST 28147-89 and STB 34.101.31 (BelT)
- Modes: ECB, gamma (CTR), gamma with feedback (CFB), gamma with MAC (GOST only)
- Input manually or from a file (`.txt`, `.md`, `.go`), output can be saved to a file
- Random key generation with key length validation

## Modes

| Algorithm | Modes |
|---|---|
| GOST 28147-89 | Simple (ECB), Gamma, Gamma with feedback, Gamma with MAC |
| STB 34.101.31 | Simple (ECB), Gamma with feedback |

## Run

```bash
go run .
```

## Keybindings

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | switch fields |
| `Enter` | select value / run |
| `Ctrl+G` | generate random key (KEY field focused) |
| `Ctrl+K` | clear key |
| `Ctrl+D` | move result to input |
| `Alt+D` | clear input |
| `Esc` / `Ctrl+C` | quit |

## Tests

```bash
go test ./...
```
