# AI Guardian

**Runtime security layer per agenti AI su macOS.**

AI Guardian si mette tra te e gli agenti AI (Claude Code, Codex, ecc.) e intercetta ogni comando prima che venga eseguito. Decide se lasciarlo passare, bloccarlo, o eseguirlo in un container Docker isolato — secondo regole YAML che definisci tu.

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
  [Audit log]  ←  ogni evento registrato in ~/.guardian/audit.jsonl
```

Il daemon gira in background, avviato automaticamente al login tramite LaunchAgent macOS.

---

## Installazione

### Prerequisiti

- macOS (arm64 o x86_64)
- Go 1.21+
- Clang (incluso in Xcode Command Line Tools)
- Docker Desktop (per la sandbox — Ciclo 2)

```bash
xcode-select --install
```

### Build

```bash
git clone https://github.com/pietroperona/agent-guardian
cd agent-guardian
make all
```

Produce tre binari nella root del progetto:
- `guardian` — CLI principale
- `guardian-shim` — binario C per l'interception PATH
- `guardian-intercept.dylib` — libreria per DYLD injection (agenti senza Hardened Runtime)

### Setup

```bash
./guardian init
```

Il wizard guida nella configurazione delle regole di policy. Al termine:
- La policy viene salvata in `~/.guardian/policy.yaml`
- Il daemon viene registrato come LaunchAgent (avvio automatico al login)
- Gli shims vengono installati in `~/.guardian/shims/`

---

## Utilizzo

### Avvia un agente sotto protezione

```bash
./guardian run claude
./guardian run python3 my_agent.py
./guardian run node agent.js
```

### Esegui un comando esplicitamente in sandbox

```bash
guardian sandbox run "python3 migration.py"
guardian sandbox run --image alpine:3.20 --network bridge "bash deploy.sh"
```

Output:

```
[⬡ sandbox] python3 migration.py — script Python eseguito in ambiente isolato
Hello, World!

[⬡ sandbox] completato con exit code 0
```

### Verifica che tutto funzioni

```bash
./guardian doctor
```

Output:

```
Guardian — diagnostica:
  ✓ directory ~/.guardian
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
./guardian logs
./guardian logs --limit 20
./guardian logs --decision block
./guardian logs --decision sandbox
```

---

## Gestione policy

```bash
# Mostra tutte le regole
guardian policy list

# Attiva/disattiva una regola
guardian policy toggle block_sudo

# Aggiungi una regola custom
guardian policy add

# Rimuovi una regola
guardian policy remove block_sudo
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
guardian init                     Installa Guardian, esegui il wizard di policy
guardian init --yes               Installa con tutti i default senza wizard
guardian start                    Avvia il daemon in foreground
guardian run <agente> [args...]   Avvia un agente AI sotto protezione
guardian sandbox run <cmd>        Esegui un comando esplicitamente in sandbox Docker
guardian sandbox run --image <i>  Specifica l'immagine Docker da usare
guardian sandbox run --network <n> Specifica la modalità rete (none/bridge)
guardian policy list              Mostra tutte le regole
guardian policy toggle <id>       Attiva/disattiva una regola
guardian policy add               Aggiungi una regola interattivamente
guardian policy remove <id>       Rimuovi una regola
guardian logs                     Mostra l'audit trail
guardian logs --decision sandbox  Mostra solo eventi sandbox
guardian doctor                   Diagnostica installazione (include check Docker)
guardian uninstall                Rimuovi Guardian dal sistema
guardian help                     Mostra questo help
```

---

## Limitazioni note

- **Claude Code** (e altri agenti con Hardened Runtime) non sono intercettabili via `DYLD_INSERT_LIBRARIES`. AI Guardian usa PATH shims come approccio principale, che funziona con qualsiasi agente.
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
