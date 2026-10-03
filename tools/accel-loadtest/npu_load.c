// Keeps the NPU busy with an INT8 matrix multiplication (no model file needed), for N seconds.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include "rknn_matmul_api.h"

static double now(void) { struct timespec t; clock_gettime(CLOCK_MONOTONIC, &t); return t.tv_sec + t.tv_nsec / 1e9; }

int main(int argc, char **argv) {
    int secs = argc > 1 ? atoi(argv[1]) : 15;
    int M = 2048, K = 2048, N = 2048;
    rknn_matmul_ctx ctx;
    rknn_matmul_info info;
    rknn_matmul_io_attr io;
    memset(&info, 0, sizeof info);
    info.M = M; info.K = K; info.N = N;
    info.type = RKNN_INT8_MM_INT8_TO_INT32;
    int ret = rknn_matmul_create(&ctx, &info, &io);
    if (ret) { fprintf(stderr, "rknn_matmul_create failed: %d\n", ret); return 1; }
    rknn_tensor_mem *A = rknn_create_mem(ctx, io.A.size), *B = rknn_create_mem(ctx, io.B.size), *C = rknn_create_mem(ctx, io.C.size);
    memset(A->virt_addr, 1, io.A.size);
    memset(B->virt_addr, 1, io.B.size);
    rknn_matmul_set_io_mem(ctx, A, &io.A);
    rknn_matmul_set_io_mem(ctx, B, &io.B);
    rknn_matmul_set_io_mem(ctx, C, &io.C);
    double t0 = now(); long n = 0;
    while (now() - t0 < secs) { if (rknn_matmul_run(ctx)) { fprintf(stderr, "run failed\n"); return 2; } n++; }
    double dt = now() - t0;
    printf("npu_load: %ld matmuls %dx%dx%d in %.1fs = %.1f TOPS\n", n, M, K, N, dt, 2.0 * M * K * N * n / dt / 1e12);
    rknn_destroy_mem(ctx, A); rknn_destroy_mem(ctx, B); rknn_destroy_mem(ctx, C);
    rknn_matmul_destroy(ctx);
    return 0;
}
