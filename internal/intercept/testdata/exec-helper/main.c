/*
 * exec-helper.c
 *
 * Binario C non-SIP usato negli integration test di guardian.
 * Chiama execvp() via C library (come fanno Node.js, Python, Ruby, ecc.),
 * a differenza dei binari Go che usano syscall raw bypassando libc.
 *
 * Compilazione: clang -o exec-helper main.c
 * Uso: ./exec-helper <cmd> [args...]
 */

#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <string.h>
#include <sys/wait.h>

int main(int argc, char *argv[])
{
    if (argc < 2) {
        fprintf(stderr, "usage: exec-helper <cmd> [args...]\n");
        return 1;
    }

    pid_t pid = fork();
    if (pid < 0) {
        perror("fork");
        return 1;
    }

    if (pid == 0) {
        /* processo figlio: esegue il comando via execvp (C library) */
        execvp(argv[1], &argv[1]);
        /* se arriviamo qui, execvp è fallita */
        perror("execvp");
        _exit(1);
    }

    /* processo padre: attende il figlio */
    int status;
    waitpid(pid, &status, 0);
    return WIFEXITED(status) ? WEXITSTATUS(status) : 1;
}
