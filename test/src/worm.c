#include <err.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

#include <sys/types.h>
#include <sys/wait.h>

void worm(int);

int main(int argc, char *argv[]) {
  int sleepfor = 10;
  int i;

  if (argc > 1) {
    sleepfor = atoi(argv[1]);
  }

  for (i = 2; i > 0; i--) {
    worm(3);
  }
  (void)sleep(sleepfor);
  return 0;
}

void worm(int n) {
  pid_t pid, pid1;

  n--;

  if (n <= 0)
    return;

  pid = fork();
  if (pid != 0) {
    return;
  }

  if (setsid() == -1) {
    warn("setsid");
  }

  pid1 = fork();
  if (pid1 != 0) {
    (void)waitpid(pid, NULL, 0);
    _exit(0);
  }

  worm(n);
}
