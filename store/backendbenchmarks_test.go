package store

import (
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// This file contains some tests which can be used to benchmark
// performance against real AWS API's.  Since this requires provisioned
// infra and authed AWS user/role, these tests are disabled during automated testing.

// To enable set the testing flag backendbenchmark (go test -benchmark)

const (
	KeysPerService = 15
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

var benchmarkEnabled bool

func init() {
	flag.BoolVar(&benchmarkEnabled, "benchmark", false, "run backend benchmarks")
}

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func benchmarkStore(t *testing.T, store Store, services []string) {
	setupStore(t, store, services)
	defer cleanupStore(t, store, services)

	concurrentExecs := []int{1, 10, 500, 1000}

	for _, concurrency := range concurrentExecs {
		var wg sync.WaitGroup
		start := time.Now()

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go emulateExec(t, &wg, store, services)

		}
		wg.Wait()
		elapsed := time.Now().Sub(start)
		t.Logf("Concurrently started %d services in %s", concurrency, elapsed)
	}
}

func emulateExec(t *testing.T, wg *sync.WaitGroup, s Store, services []string) error {
	defer wg.Done()
	// Exec calls ListRaw once per service specified
	for _, service := range services {
		_, err := s.ListRaw(service)
		if err != nil {
			t.Logf("Failed to execute ListRaw: %s", err)
			return err
		}
	}
	return nil
}

func TestS3StoreConcurrency(t *testing.T) {
	if !benchmarkEnabled {
		t.SkipNow()
	}
	s, _ := NewS3Store(10)
	benchmarkStore(t, s, []string{"foo"})
}

func TestSSMConcurrency(t *testing.T) {
	if !benchmarkEnabled {
		t.SkipNow()
	}
	s, _ := NewSSMStore(10)
	benchmarkStore(t, s, []string{"foo"})
}

func setupStore(t *testing.T, store Store, services []string) {
	// populate the store for services listed
	for _, service := range services {
		for i := 0; i < KeysPerService; i++ {
			key := fmt.Sprintf("var%d", i)
			id := SecretId{
				Service: service,
				Key:     key,
			}

			store.Write(id, RandStringRunes(100))
		}
	}
}

func cleanupStore(t *testing.T, store Store, services []string) {
	for _, service := range services {
		for i := 0; i < KeysPerService; i++ {
			key := fmt.Sprintf("var%d", i)
			id := SecretId{
				Service: service,
				Key:     key,
			}

			store.Delete(id)
		}
	}
}
