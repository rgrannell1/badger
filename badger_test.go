package main

import "testing"

func BenchmarkComputeBlur(bench *testing.B) {
	// run the Fib function b.N times
	for n := 0; n < bench.N; n++ {
		ComputeBlur("/home/rg/Desktop/test.JPG")
	}
}
