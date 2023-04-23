package util

/*
#include <time.h>
static unsigned long long get_nanos(void) {
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return (unsigned long long)ts.tv_sec * 1000000000UL + ts.tv_nsec;
}
*/
import "C"

func GetMonoTimeNanos() int64 {
	return int64(C.get_nanos())
}
