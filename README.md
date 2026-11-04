# go-cipher

Terminal UI (Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea)) for encrypting
and decrypting text and files. Implements **GOST 28147-89** and **STB 34.101.31 (BelT)**,
plus an asymmetric tab for **Rabin** and a hash tab for **GOST R 34.11-2012 (Streebog)** and **SHA-1**.

![go-cipher TUI](Reports/Lab1/img/tui-cipher.png)

## Features

- Four tabs: GOST 28147-89, STB 34.101.31 (BelT), Rabin, Hash
- Symmetric modes: ECB, gamma (CTR), gamma with feedback (CFB), gamma with MAC (GOST only)
- Asymmetric workflow: keypair generation, public-key encryption, private-key decryption
- Hash workflow: Streebog-512, Streebog-256 and SHA-1 with an expected-value verdict and an avalanche check
- Private key is masked in its own field; `Ctrl+R` reveals it
- Input manually or from a file (`.txt`, `.md`, `.go`), output can be saved to a file
- Random key generation with key length validation

## Modes

| Algorithm | Modes |
|---|---|
| GOST 28147-89 | Simple (ECB), Gamma, Gamma with feedback, Gamma with MAC |
| STB 34.101.31 | Simple (ECB), Gamma with feedback |

| Hash algorithm | Digest |
|---|---|
| GOST R 34.11-2012 | 512, 256 bit |
| SHA-1 | 160 bit |

## Run

```bash
go run .
```

## Keybindings

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | switch fields |
| `Enter` | select value / run |
<<<<<<< HEAD
| `Ctrl+G` | generate random key (KEY field focused) |
| `Ctrl+K` | clear key |
| `Ctrl+D` | move result to input |
=======
| `Ctrl+G` | generate keys |
| `Ctrl+R` | reveal/hide private key (asymmetric tabs) |
| `Ctrl+K` | clear focused key field |
| `Ctrl+D` | move result to input / digest to expected (hash tab) |
>>>>>>> 396d57f (feat: add Streebog and SHA-1 hashing with Hash tab and lab 5 report)
| `Alt+D` | clear input |
| `Enter` | switch algorithm/source (hash tab) or run |
| `Esc` / `Ctrl+C` | quit |

## Tests

```bash
go test ./...
```
