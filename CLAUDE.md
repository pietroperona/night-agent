# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**AI Guardian** è un CLI tool per macOS che funge da runtime security layer per agenti AI (Claude Code, Codex, ecc.). Intercetta e governa le azioni degli agenti prima che vengano eseguite. Il progetto è attualmente in fase di pianificazione/design — il codice è ancora da scrivere.

Documento di specifica completo: [docs/ai-agent-guardian-mvp-plan.md](docs/ai-agent-guardian-mvp-plan.md)

## Tech Stack Pianificato

- **Linguaggio**: Go (single binary, process handling, cross-platform)
- **Config**: YAML
- **Logging**: SQLite o JSONL
- **Packaging**: Homebrew tap
- **Sandbox (Cycle 2)**: Docker Desktop

## Architettura — 5 Layer Logici

```
Agente AI
    ↓
[Interception Layer]  ← cattura shell commands, file ops, git ops, processi
    ↓
[Policy Engine]       ← valuta regole YAML → decisione: allow / block / ask / sandbox
    ↓
[Execution Layer]     ← esegue azioni approvate (locale o isolato)
    ↓
[Audit Layer]         ← log con timestamp, decisione, motivo, contesto
    ↑
[Config & UX Layer]   ← CLI per init, logs, policy edit, approve
```

## CLI Commands Pianificati

```bash
guardian init          # Inizializza progetto con policy di default
guardian run           # Esegue agente con protezione attiva
guardian logs          # Visualizza audit trail
guardian policy edit   # Modifica regole di policy
guardian doctor        # Diagnostica e verifica setup
guardian sandbox run   # Esegue in container isolato (Cycle 2)
```

## Policy Model

```yaml
rules:
  - id: block_sudo
    when: {action_type: shell, command_matches: ["sudo *"]}
    decision: block
    reason: "sudo disabled"

  - id: ask_git_push_main
    when: {action_type: git, command_matches: ["git push origin main"]}
    decision: ask
    reason: "push to protected branch"
```

Decisioni possibili: `allow`, `block`, `ask`, `sandbox`

## Roadmap

- **Cycle 1 (MVP)**: Policy engine rule-based, intercettazione shell + git, audit log
- **Cycle 2**: Docker sandbox integration, network isolation
- **Cycle 3**: Risk scoring, anomaly detection, policy suggestions

## Repository & Git Workflow

- **Remote**: [github.com/pietroperona/agent-guardian](https://github.com/pietroperona/agent-guardian)
- **Branch principali**:
  - `main` — produzione, solo merge da feature branch o staging
  - `develop` — sviluppo locale
  - `staging` — ambiente di staging (futuro)
- **Feature branch**: ogni funzione specifica ha il suo branch (`feature/nome`), poi merge su `main`
- **Commit**: non citare mai il nome di strumenti AI nei messaggi di commit

## Approccio di Sviluppo — TDD

Tutto lo sviluppo segue **Test-Driven Development**:

1. Scrivi il test che descrive il comportamento atteso (red)
2. Scrivi il codice minimo per farlo passare (green)
3. Refactoring se necessario (refactor)

In Go, ogni package ha il suo file `_test.go`. Eseguire i test con:

```bash
go test ./...              # tutti i test
go test ./internal/policy  # test di un package specifico
go test -run TestNomeFunzione ./...  # singolo test
```

## Principi di Design Fondamentali

1. Controlla **azioni**, non intenzioni
2. **Determinismo** prima di intelligenza — no ML per decisioni hard
3. **Explainability** — l'utente deve sempre capire perché qualcosa è bloccato
4. **Safe failure mode** — in caso di errore del guardian, blocca (non permette)
5. Security by default, friction minimale
6. Framework-agnostic — funziona con qualsiasi agente AI

## Superfici da Proteggere (Scope MVP)

- Shell commands pericolosi (`rm -rf`, `curl | bash`, `chmod 777`, ecc.)
- File operations su path sensibili (`~/.ssh`, `~/.aws`, `.env`)
- Git operations (`git push --force`, push su main/master)
- Installazione package/processi non autorizzati
