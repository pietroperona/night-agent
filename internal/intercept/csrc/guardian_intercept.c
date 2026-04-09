/*
 * guardian_intercept.c
 *
 * Libreria DYLD_INSERT_LIBRARIES per macOS.
 * Usa DYLD_INTERPOSE (il meccanismo Apple ufficiale) per intercettare
 * execve() e posix_spawn() dall'interno del processo agente.
 *
 * DYLD_INTERPOSE è necessario su macOS arm64: la semplice ridefinizione di
 * execve non funziona perché dyld usa two-level namespace e flat redirection.
 *
 * Compilazione:
 *   clang -dynamiclib -o guardian-intercept.dylib guardian_intercept.c \
 *         -Wall -Wextra -Wno-unused-parameter
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>
#include <dlfcn.h>
#include <spawn.h>
#include <sys/socket.h>
#include <sys/un.h>

/* ---------- DYLD_INTERPOSE macro ------------------------------------- */
/*
 * Dichiara una relazione di interposizione: ogni chiamata a _replacee
 * viene reindirizzata a _replacement da dyld al momento del caricamento.
 */
/* Forma canonica: entrambi gli attributi prima del tipo struct */
#define DYLD_INTERPOSE(_replacement, _replacee) \
    static __attribute__((used)) __attribute__((section("__DATA,__interpose"))) \
    struct { const void *replacement; const void *replacee; } \
    _interpose_##_replacee = { \
        (const void *)(unsigned long)&(_replacement), \
        (const void *)(unsigned long)&(_replacee)  \
    }

/* ---------- debug ---------------------------------------------------- */

static int debug_enabled(void)
{
    return getenv("GUARDIAN_DEBUG") != NULL;
}

#define DBG(fmt, ...) do { \
    if (debug_enabled()) \
        fprintf(stderr, "[guardian] " fmt "\n", ##__VA_ARGS__); \
} while (0)

__attribute__((constructor))
static void guardian_init(void)
{
    DBG("dylib caricata (DYLD_INTERPOSE). socket=%s",
        getenv("GUARDIAN_SOCKET") ? getenv("GUARDIAN_SOCKET") : "(non impostato)");
}

/* ---------- comunicazione con il daemon ------------------------------ */

static int guardian_check(const char *path, char *const argv[])
{
    if (getenv("GUARDIAN_BYPASS") != NULL) return 0;

    const char *sock_path = getenv("GUARDIAN_SOCKET");
    if (sock_path == NULL || sock_path[0] == '\0') return 0;

    DBG("hook: %s", path);

    /* costruisci "path arg1 arg2 ..." */
    char command[4096] = {0};
    strlcpy(command, path, sizeof(command));
    if (argv != NULL) {
        for (int i = 1; argv[i] != NULL; i++) {
            strlcat(command, " ", sizeof(command));
            strlcat(command, argv[i], sizeof(command));
        }
    }

    /* escape JSON minimale */
    char escaped[8192] = {0};
    int j = 0;
    for (int i = 0; command[i] && j < (int)sizeof(escaped) - 2; i++) {
        if (command[i] == '"' || command[i] == '\\')
            escaped[j++] = '\\';
        escaped[j++] = command[i];
    }

    char payload[8192];
    snprintf(payload, sizeof(payload),
        "{\"command\":\"%s\",\"work_dir\":\"\",\"agent_name\":\"\"}\n",
        escaped);

    int fd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (fd < 0) { DBG("socket() fallita: safe failure"); return 1; }

    struct sockaddr_un addr;
    memset(&addr, 0, sizeof(addr));
    addr.sun_family = AF_UNIX;
    strlcpy(addr.sun_path, sock_path, sizeof(addr.sun_path));

    if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        close(fd);
        DBG("connect() fallita (%s): safe failure", sock_path);
        return 1;
    }

    if (write(fd, payload, strlen(payload)) < 0) {
        close(fd);
        DBG("write() fallita: safe failure");
        return 1;
    }

    char response[2048] = {0};
    ssize_t n = read(fd, response, sizeof(response) - 1);
    close(fd);

    if (n <= 0) { DBG("nessuna risposta: safe failure"); return 1; }

    DBG("risposta: %s", response);

    if (strstr(response, "\"decision\":\"block\"") != NULL) {
        const char *reason_key = "\"reason\":\"";
        char *rs = strstr(response, reason_key);
        if (rs) {
            rs += strlen(reason_key);
            char *re = strchr(rs, '"');
            if (re) {
                char reason[512] = {0};
                size_t len = (size_t)(re - rs);
                if (len >= sizeof(reason)) len = sizeof(reason) - 1;
                memcpy(reason, rs, len);
                fprintf(stderr, "guardian: bloccato — %s\n", reason);
            }
        } else {
            fprintf(stderr, "guardian: comando bloccato dalla policy\n");
        }
        return 1;
    }

    if (strstr(response, "\"decision\":\"ask\"") != NULL) {
        /* TODO Cycle 2: prompt interattivo. Per ora logga e permette. */
        fprintf(stderr, "guardian: [ask] %s\n", command);
        return 0;
    }

    return 0;
}

/* ---------- sostituzioni --------------------------------------------- */

static int guardian_execve(const char *path, char *const argv[], char *const envp[])
{
    if (guardian_check(path, argv)) {
        errno = EPERM;
        return -1;
    }
    return execve(path, argv, envp);
}

static int guardian_posix_spawn(
    pid_t *pid, const char *path,
    const posix_spawn_file_actions_t *fa,
    const posix_spawnattr_t *attr,
    char *const argv[], char *const envp[])
{
    if (guardian_check(path, argv)) {
        errno = EPERM;
        return EPERM;
    }
    return posix_spawn(pid, path, fa, attr, argv, envp);
}

static int guardian_posix_spawnp(
    pid_t *pid, const char *file,
    const posix_spawn_file_actions_t *fa,
    const posix_spawnattr_t *attr,
    char *const argv[], char *const envp[])
{
    if (guardian_check(file, argv)) {
        errno = EPERM;
        return EPERM;
    }
    return posix_spawnp(pid, file, fa, attr, argv, envp);
}

/* ---------- dichiarazioni DYLD_INTERPOSE ----------------------------- */

DYLD_INTERPOSE(guardian_execve,       execve);
DYLD_INTERPOSE(guardian_posix_spawn,  posix_spawn);
DYLD_INTERPOSE(guardian_posix_spawnp, posix_spawnp);
