#include <string.h>
#include <assert.h>
#include <stdio.h>
#include <cuda_runtime.h>
#include <iostream>
#include <cuda.h>

using namespace std;

int main(){
    struct cudaDeviceProp devProp;
    size_t stackSize, fifoSize, heapSize, sharedMemPerBlock, free, total;

    cudaSetDevice(0);
    cudaMemGetInfo(&free, &total);
    cudaGetDeviceProperties(&devProp, 0);
    cudaDeviceGetLimit(&heapSize, cudaLimitMallocHeapSize);
    cudaDeviceGetLimit(&fifoSize, cudaLimitPrintfFifoSize);
    cudaDeviceGetLimit(&stackSize, cudaLimitStackSize);

   cout << free << "\n" << total << "\n" << devProp.multiProcessorCount << endl;

    return 0;
}
