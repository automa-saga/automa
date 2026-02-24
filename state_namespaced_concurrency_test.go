package automa

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSyncNamespacedStateBag_Concurrent_Local verifies thread-safe access to Local() namespace
func TestSyncNamespacedStateBag_Concurrent_Local(t *testing.T) {
	ns := NewNamespacedStateBag(nil, nil)
	const goroutines = 100
	const operations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				// Concurrent writes to local namespace
				local := ns.Local()
				local.Set(Key("key"), id)

				// Concurrent reads
				_, ok := local.Get(Key("key"))
				assert.True(t, ok)
			}
		}(i)
	}

	wg.Wait()

	// Verify state is consistent after concurrent access
	local := ns.Local()
	assert.NotNil(t, local)
	_, ok := local.Get(Key("key"))
	assert.True(t, ok)
}

// TestSyncNamespacedStateBag_Concurrent_Global verifies thread-safe access to Global() namespace
func TestSyncNamespacedStateBag_Concurrent_Global(t *testing.T) {
	ns := NewNamespacedStateBag(nil, nil)
	const goroutines = 100
	const operations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				// Concurrent writes to global namespace
				global := ns.Global()
				global.Set(Key("global-key"), id)

				// Concurrent reads
				_, ok := global.Get(Key("global-key"))
				assert.True(t, ok)
			}
		}(i)
	}

	wg.Wait()

	// Verify state is consistent after concurrent access
	global := ns.Global()
	assert.NotNil(t, global)
	_, ok := global.Get(Key("global-key"))
	assert.True(t, ok)
}

// TestSyncNamespacedStateBag_Concurrent_WithNamespace verifies thread-safe access to custom namespaces
func TestSyncNamespacedStateBag_Concurrent_WithNamespace(t *testing.T) {
	ns := NewNamespacedStateBag(nil, nil)
	const goroutines = 100
	const operations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				// Concurrent access to same custom namespace
				custom := ns.WithNamespace("shared-ns")
				custom.Set(Key("custom-key"), id)

				// Concurrent reads
				_, ok := custom.Get(Key("custom-key"))
				assert.True(t, ok)
			}
		}(i)
	}

	wg.Wait()

	// Verify custom namespace exists and is accessible
	custom := ns.WithNamespace("shared-ns")
	assert.NotNil(t, custom)
	_, ok := custom.Get(Key("custom-key"))
	assert.True(t, ok)
}

// TestSyncNamespacedStateBag_Concurrent_MultipleNamespaces verifies thread-safe creation of multiple custom namespaces
func TestSyncNamespacedStateBag_Concurrent_MultipleNamespaces(t *testing.T) {
	ns := NewNamespacedStateBag(nil, nil)
	const goroutines = 50
	const namespacesPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < namespacesPerGoroutine; j++ {
				// Create unique namespace per goroutine
				nsName := Key("ns-" + string(rune(id)) + "-" + string(rune(j)))
				custom := ns.WithNamespace(string(nsName))
				custom.Set(Key("data"), id)

				// Verify data
				val, ok := custom.Get(Key("data"))
				assert.True(t, ok)
				assert.Equal(t, id, val)
			}
		}(i)
	}

	wg.Wait()

	// Verify all namespaces were created
	// We expect goroutines * namespacesPerGoroutine unique namespaces
	ns.mu.RLock()
	customCount := len(ns.custom)
	ns.mu.RUnlock()

	assert.Equal(t, goroutines*namespacesPerGoroutine, customCount)
}

// TestSyncNamespacedStateBag_Concurrent_Clone verifies thread-safe cloning
func TestSyncNamespacedStateBag_Concurrent_Clone(t *testing.T) {
	ns := NewNamespacedStateBag(nil, nil)

	// Populate with data
	ns.Local().Set("local-key", "local-value")
	ns.Global().Set("global-key", "global-value")
	ns.WithNamespace("custom").Set("custom-key", "custom-value")

	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	clones := make([]NamespacedStateBag, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			// Concurrent clones
			cloned, err := ns.Clone()
			require.NoError(t, err)
			clones[id] = cloned

			// Verify cloned data
			assert.Equal(t, "local-value", cloned.Local().String("local-key"))
			assert.Equal(t, "global-value", cloned.Global().String("global-key"))
			assert.Equal(t, "custom-value", cloned.WithNamespace("custom").String("custom-key"))
		}(i)
	}

	wg.Wait()

	// Verify all clones are independent
	for i, cloned := range clones {
		assert.NotNil(t, cloned, "clone %d should not be nil", i)

		// Modify clone
		cloned.Local().Set("modified", i)

		// Verify original is not affected
		_, ok := ns.Local().Get("modified")
		assert.False(t, ok, "original should not see clone modifications")
	}
}

// TestSyncNamespacedStateBag_Concurrent_Merge verifies thread-safe merging
func TestSyncNamespacedStateBag_Concurrent_Merge(t *testing.T) {
	ns1 := NewNamespacedStateBag(nil, nil)
	ns1.Local().Set("key1", "value1")
	ns1.Global().Set("global1", "gvalue1")

	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Create a new state bag to merge
			other := NewNamespacedStateBag(nil, nil)
			other.Local().Set(Key("local-"+string(rune(id))), id)
			other.Global().Set(Key("global-"+string(rune(id))), id)

			// Concurrent merge
			_, err := ns1.Merge(other)
			require.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify merged data contains original keys
	assert.Equal(t, "value1", ns1.Local().String("key1"))
	assert.Equal(t, "gvalue1", ns1.Global().String("global1"))
}

// TestSyncNamespacedStateBag_Concurrent_MixedOperations verifies thread-safety with mixed operations
func TestSyncNamespacedStateBag_Concurrent_MixedOperations(t *testing.T) {
	ns := NewNamespacedStateBag(nil, nil)
	const goroutines = 50
	const operations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 5) // 5 different operation types

	// Concurrent Local() access
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				ns.Local().Set(Key("local"), id)
				ns.Local().Get(Key("local"))
			}
		}(i)
	}

	// Concurrent Global() access
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				ns.Global().Set(Key("global"), id)
				ns.Global().Get(Key("global"))
			}
		}(i)
	}

	// Concurrent WithNamespace() access
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				ns.WithNamespace("custom").Set(Key("custom"), id)
				ns.WithNamespace("custom").Get(Key("custom"))
			}
		}(i)
	}

	// Concurrent Clone()
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				_, _ = ns.Clone()
			}
		}()
	}

	// Concurrent Merge()
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				other := NewNamespacedStateBag(nil, nil)
				other.Local().Set(Key("merge"), id)
				_, _ = ns.Merge(other)
			}
		}(i)
	}

	wg.Wait()

	// Verify state is still accessible after concurrent operations
	assert.NotNil(t, ns.Local())
	assert.NotNil(t, ns.Global())
	assert.NotNil(t, ns.WithNamespace("custom"))
}

// TestSyncNamespacedStateBag_Concurrent_LocalLazyInit verifies thread-safe lazy initialization of Local()
func TestSyncNamespacedStateBag_Concurrent_LocalLazyInit(t *testing.T) {
	// Create without local namespace (nil)
	ns := NewNamespacedStateBag(nil, nil)

	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			// All goroutines concurrently trigger lazy init
			local := ns.Local()
			local.Set(Key("key"), id)
		}(i)
	}

	wg.Wait()

	// Verify local was initialized (exactly once is guaranteed by sync.Once)
	assert.NotNil(t, ns.local)

	// Verify local is accessible and contains data
	local := ns.Local()
	_, ok := local.Get(Key("key"))
	assert.True(t, ok)
}

// TestSyncNamespacedStateBag_Concurrent_MergeMultipleSources verifies thread-safe merge from multiple sources
func TestSyncNamespacedStateBag_Concurrent_MergeMultipleSources(t *testing.T) {
	target := NewNamespacedStateBag(nil, nil)
	target.Local().Set("target-key", "target-value")

	const sources = 50

	var wg sync.WaitGroup
	wg.Add(sources)

	for i := 0; i < sources; i++ {
		go func(id int) {
			defer wg.Done()

			source := NewNamespacedStateBag(nil, nil)
			source.Local().Set(Key("source-"+string(rune(id))), id)
			source.Global().Set(Key("global-"+string(rune(id))), id)
			source.WithNamespace("custom").Set(Key("custom-"+string(rune(id))), id)

			_, err := target.Merge(source)
			require.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify target still has original key
	assert.Equal(t, "target-value", target.Local().String("target-key"))
}
