# go-cipher

Terminal UI (Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea)) for encrypting
and decrypting text and files. Implements **GOST 28147-89** and **STB 34.101.31 (BelT)**,
with an asymmetric tab for **Rabin** and a **Stego** tab
for hiding text in the DCT coefficients of a JPEG image.

![go-cipher TUI](Reports/Lab1/img/tui-cipher.png)

## Features

- Four tabs: GOST 28147-89, STB 34.101.31 (BelT), Rabin, Stego
- Symmetric modes: ECB, gamma (CTR), gamma with feedback (CFB), gamma with MAC (GOST only)
- Asymmetric workflow: keypair generation, public-key encryption, private-key decryption
- Private key is masked in its own field; `Ctrl+R` reveals it
- Input manually or from a file (`.txt`, `.md`, `.go`), output can be saved to a file
- Stego tab: quantisation index modulation in JPEG DCT coefficients, PNG container, and
  original / stego / amplified difference previews
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
| `Ctrl+G` | generate keys |
| `Ctrl+R` | reveal/hide private key (asymmetric tabs) |
| `Ctrl+K` | clear focused key field |
| `Ctrl+D` | move result to input / extracted text to message |
| `Alt+D` | clear input |
| `Esc` / `Ctrl+C` | quit |

## Tests

```bash
go test ./...
```
