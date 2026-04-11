# Night Agent

**Runtime security layer per agenti AI su macOS.**

Night Agent si mette tra te e gli agenti AI (Claude Code, Codex, ecc.) e intercetta ogni comando prima che venga eseguito. Decide se lasciarlo passare, bloccarlo, o eseguirlo in un container Docker isolato — secondo regole YAML che definisci tu.

---

## Come funziona

```
Agente AI (Claude Code, python3, bash...)
        ↓
  [PATH shims]  ←  ogni comando passa per guardian-shim
        ↓
  [Policy engine]  ←  valuta le regole YAML
        ↓
  allow   → esegue il comando sull'host
  block   → blocca con messaggio
  sandbox → esegue in container Docker isolato
        ↓
  [Audit log]  ←  ogni evento registrato in ~/.night-agent/audit.jsonl
```

Il daemon gira in background, avviato automaticamente al login tramite LaunchAgent macOS.

---

## Installazione

### Via Homebrew (consigliato)

```bash
brew tap pietroperona/night-agent
brew install night-agent
```

Poi inizializza:

```bash
night-agent init
```

Per le funzionalità sandbox installa [Docker Desktop](https://www.docker.com/products/docker-desktop/) e avvialo almeno una volta.

### Build da sorgente

#### Prerequisiti

- macOS (arm64 o x86_64)
- Go 1.21+
- Clang (incluso in Xcode Command Line Tools)
- Docker Desktop (per la sandbox — Ciclo 2)

```bash
xcode-select --install
```

#### Build

```bash
git clone https://github.com/pietroperona/night-agent
cd night-agent
make all
```

Produce tre binari nella root del progetto:

- `night-agent` — CLI principale
- `guardian-shim` — binario C per l'interception PATH
- `guardian-intercept.dylib` — libreria per DYLD injection (agenti senza Hardened Runtime)

#### Setup

```bash
./night-agent init
```

Il wizard guida nella configurazione delle regole di policy. Al termine:
- La policy viene salvata in `~/.night-agent/policy.yaml`
- Il daemon viene registrato come LaunchAgent (avvio automatico al login)
- Gli shims vengono installati in `~/.night-agent/shims/`

---

## Utilizzo

### Avvia un agente sotto protezione

```bash
./night-agent run claude
./night-agent run python3 my_agent.py
./night-agent run node agent.js
```

### Esegui un comando esplicitamente in sandbox

```bash
night-agent sandbox run "python3 migration.py"
night-agent sandbox run --image alpine:3.20 --network bridge "bash deploy.sh"
```

Output:

```
[⬡ sandbox] python3 migration.py — script Python eseguito in ambiente isolato
Hello, World!

[⬡ sandbox] completato con exit code 0
```

### Verifica che tutto funzioni

```bash
./night-agent doctor
```

Output:

```
Guardian — diagnostica:
  ✓ directory ~/.night-agent
  ✓ policy.yaml
  ✓ hook shell (.zshrc)
  ✓ daemon in esecuzione

Sandbox (Ciclo 2):
  ✓ Docker installato
  ✓ Docker daemon in esecuzione

tutto ok — guardian è operativo
```

### Vedi cosa sta succedendo

```bash
./night-agent logs
./night-agent logs --limit 20
./night-agent logs --decision block
./night-agent logs --decision sandbox
```

---

## Gestione policy

```bash
# Mostra tutte le regole
night-agent policy list

# Attiva/disattiva una regola
night-agent policy toggle block_sudo

# Aggiungi una regola custom
night-agent policy add

# Rimuovi una regola
night-agent policy remove block_sudo
```

### Esempio policy.yaml

```yaml
version: 1
rules:
  - id: block_sudo
    when:
      action_type: shell
      command_matches: ["sudo *", "*/sudo *"]
    match_type: glob
    decision: block
    reason: "sudo disabilitato per gli agenti AI"

  - id: block_rm_rf
    when:
      action_type: shell
      command_matches: ["rm -rf *", "rm -fr *"]
    match_type: glob
    decision: ask
    reason: "cancellazione ricorsiva richiede conferma"

  - id: ask_git_push_main
    when:
      action_type: git
      command_matches: ["git push * main", "git push * master", "git push --force *"]
    match_type: glob
    decision: ask
    reason: "push su branch protetto richiede conferma"

  # Regole sandbox — esegue in Docker isolato invece di bloccare
  - id: sandbox_python_scripts
    when:
      action_type: shell
      command_matches: ["python *.py", "python3 *.py"]
    match_type: glob
    decision: sandbox
    sandbox:
      image: "python:3.12-alpine"
      network: "none"
    reason: "script Python eseguito in ambiente isolato"

  - id: sandbox_shell_scripts
    when:
      action_type: shell
      command_matches: ["bash *.sh", "sh *.sh"]
    match_type: glob
    decision: sandbox
    sandbox:
      image: "alpine:3.20"
      network: "none"
    reason: "script shell eseguito in ambiente isolato"
```

Decisioni disponibili: `allow`, `block`, `ask` (trattato come `block` a runtime), `sandbox`.

### Configurazione sandbox per regola

Il campo `sandbox` è opzionale e si applica solo alle regole con `decision: sandbox`:

| Campo     | Default       | Valori                    |
|-----------|---------------|---------------------------|
| `image`   | `alpine:3.20` | qualsiasi immagine Docker |
| `network` | `none`        | `none`, `bridge`          |

Il workspace corrente viene montato automaticamente come `/workspace` nel container.

---

## Comandi disponibili

```
night-agent init                     Installa Guardian, esegui il wizard di policy
night-agent init --yes               Installa con tutti i default senza wizard
night-agent start                    Avvia il daemon in foreground
night-agent run <agente> [args...]   Avvia un agente AI sotto protezione
night-agent sandbox run <cmd>        Esegui un comando esplicitamente in sandbox Docker
night-agent sandbox run --image <i>  Specifica l'immagine Docker da usare
night-agent sandbox run --network <n> Specifica la modalità rete (none/bridge)
night-agent policy list              Mostra tutte le regole
night-agent policy toggle <id>       Attiva/disattiva una regola
night-agent policy add               Aggiungi una regola interattivamente
night-agent policy remove <id>       Rimuovi una regola
night-agent logs                     Mostra l'audit trail
night-agent logs --decision sandbox  Mostra solo eventi sandbox
night-agent doctor                   Diagnostica installazione (include check Docker)
night-agent uninstall                Rimuovi Guardian dal sistema
night-agent help                     Mostra questo help
```

---

## Limitazioni note

- **Claude Code** (e altri agenti con Hardened Runtime) non sono intercettabili via `DYLD_INSERT_LIBRARIES`. Night Agent usa PATH shims come approccio principale, che funziona con qualsiasi agente.
- Intercetta comandi eseguiti via shell. Chiamate syscall dirette o chiamate native non passano per il layer di interception.
- La sandbox richiede Docker Desktop installato e in esecuzione. Se Docker non è disponibile, le regole `sandbox` fanno fail-safe su `block`.
- Richiede macOS. Linux e Windows non sono supportati.

---

## Roadmap

- **Cycle 1** ✅ — Policy engine rule-based, PATH shims, audit log, LaunchAgent
- **Cycle 2** ✅ — Docker sandbox integration, isolamento rete, routing automatico via policy
- **Cycle 3** — Risk scoring, anomaly detection, suggerimenti policy automatici

---

## Licenza

MIT
