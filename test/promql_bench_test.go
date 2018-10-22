package test

import (
	"context"
	"log"
	"path/filepath"
	"testing"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/prometheus/prometheus/storage"
)

func init() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}

func BenchmarkEvaluations(b *testing.B) {
	files, err := filepath.Glob("benchdata/*.test")
	if err != nil {
		b.Fatal(err)
	}
	testLoad, err := newTestFromFile(b, "benchdata/load.test")
	if err != nil {
		b.Errorf("error creating test for %s: %s", "benchdata/load.test", err)
	}
	testLoad.Run()

	for _, fn := range files {
		if fn == "benchdata/load.test" {
			continue
		}
		// Create swappable storages
		storageA := &SwappableStorage{}
		storageB := &SwappableStorage{}

		// Create API for the storage engine
		srv, stopChan := startAPIForTest(storageA, ":8083")
		srv2, stopChan2 := startAPIForTest(storageB, ":8084")
		ps := getProxyStorage(rawDoublePSConfig)

		b.Run(fn, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				test, err := newTestFromFile(b, fn)
				if err != nil {
					b.Errorf("error creating test for %s: %s", fn, err)
				}

				// set the storage
				storageA.s = testLoad.Storage()
				storageB.s = testLoad.Storage()

				lStorage := &LayeredStorage{ps, testLoad.Storage()}
				// Replace the test storage with the promxy one
				test.SetStorage(lStorage)
				test.QueryEngine().NodeReplacer = ps.NodeReplacer

				b.ResetTimer()
				err = test.Run()
				if err != nil {
					b.Errorf("error running test %s: %s", fn, err)
				}
				b.StopTimer()
				test.SetStorage(&StubStorage{})
				test.Close()
			}

		})

		// stop server
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		srv2.Shutdown(ctx)

		<-stopChan
		<-stopChan2
	}
}

// Swappable storage, to make benchmark perf bearable
type SwappableStorage struct {
	s storage.Storage
}

func (p *SwappableStorage) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	return p.s.Querier(ctx, mint, maxt)
}
func (p *SwappableStorage) StartTime() (int64, error) {
	return p.s.StartTime()
}
func (p *SwappableStorage) Appender() (storage.Appender, error) {
	return p.s.Appender()
}
func (p *SwappableStorage) Close() error {
	return p.s.Close()
}

type StubStorage struct{}

func (p *StubStorage) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	return nil, nil
}
func (p *StubStorage) StartTime() (int64, error) {
	return 0, nil
}
func (p *StubStorage) Appender() (storage.Appender, error) {
	return nil, nil
}
func (p *StubStorage) Close() error {
	return nil
}
