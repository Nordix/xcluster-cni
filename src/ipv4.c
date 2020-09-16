#include <stdio.h>
#include <arpa/inet.h>


int
main(int argc, char* argv[])
{
	if (argc < 2) {
		printf("0.0.0.0\n");
		return 0;
	}
	unsigned char buf[sizeof(struct in6_addr)];
	if (inet_pton(AF_INET6, argv[1], buf) == 1) {
		struct in6_addr* ap = (struct in6_addr*)buf;
		struct in_addr a4 = {ap->s6_addr32[3]};
		printf("%s\n", inet_ntoa(a4));
		return 0;
	}
	if (inet_pton(AF_INET, argv[1], buf) == 1) {
		printf("%s\n", argv[1]);
		return 0;
	}
	return -1;
}
