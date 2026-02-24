package automa

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamespacedStateBag_BasicOperations(t *testing.T) {
	t.Run("local and global isolation", func(t *testing.T) {
		ns := NewNamespacedStateBag(nil, nil)

		// Write to local
		ns.Local().Set("local-key", "local-value")

		// Write to global
		ns.Global().Set("global-key", "global-value")

		// Verify local has only local key
		val, ok := ns.Local().Get("local-key")
		assert.True(t, ok)
		assert.Equal(t, "local-value", val)

		_, ok = ns.Local().Get("global-key")
		assert.False(t, ok, "local should not see global-only keys")

		// Verify global has only global key
		val, ok = ns.Global().Get("global-key")
		assert.True(t, ok)
		assert.Equal(t, "global-value", val)

		_, ok = ns.Global().Get("local-key")
		assert.False(t, ok, "global should not see local-only keys")
	})

	t.Run("custom namespace isolation", func(t *testing.T) {
		ns := NewNamespacedStateBag(nil, nil)

		ns.WithNamespace("ns1").Set("key", "value1")
		ns.WithNamespace("ns2").Set("key", "value2")

		val1 := ns.WithNamespace("ns1").String("key")
		val2 := ns.WithNamespace("ns2").String("key")

		assert.Equal(t, "value1", val1)
		assert.Equal(t, "value2", val2)
	})

	t.Run("clone creates independent copy", func(t *testing.T) {
		original := NewNamespacedStateBag(nil, nil)

		// Set values in all namespaces
		original.Local().Set("local-key", "local-value")
		original.Global().Set("global-key", "global-value")
		original.WithNamespace("custom").Set("custom-key", "custom-value")

		// Clone the state
		cloned, err := original.Clone()
		require.NoError(t, err)
		require.NotNil(t, cloned)

		// Verify cloned values match original
		assert.Equal(t, "local-value", cloned.Local().String("local-key"))
		assert.Equal(t, "global-value", cloned.Global().String("global-key"))
		assert.Equal(t, "custom-value", cloned.WithNamespace("custom").String("custom-key"))

		// Modify original
		original.Local().Set("local-key", "modified-local")
		original.Global().Set("global-key", "modified-global")
		original.WithNamespace("custom").Set("custom-key", "modified-custom")

		// Verify clone is not affected
		assert.Equal(t, "local-value", cloned.Local().String("local-key"))
		assert.Equal(t, "global-value", cloned.Global().String("global-key"))
		assert.Equal(t, "custom-value", cloned.WithNamespace("custom").String("custom-key"))

		// Modify clone
		cloned.Local().Set("new-key", "new-value")

		// Verify original is not affected
		_, ok := original.Local().Get("new-key")
		assert.False(t, ok, "original should not see new keys in clone")
	})
}

func TestWorkflow_NamespacedState_StepIsolation(t *testing.T) {
	t.Run("steps have isolated local state", func(t *testing.T) {
		wb := NewWorkflowBuilder().WithId("test-workflow")

		// Step 1: writes to local state
		step1 := NewStepBuilder().WithId("step1").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				stp.State().Local().Set("my-key", "step1-value")
				return SuccessReport(stp)
			})

		// Step 2: writes to local state with same key
		var step2LocalValue string
		step2 := NewStepBuilder().WithId("step2").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				stp.State().Local().Set("my-key", "step2-value")
				step2LocalValue = stp.State().Local().String("my-key")
				return SuccessReport(stp)
			})

		// Step 3: verify isolation
		step3 := NewStepBuilder().WithId("step3").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				// Should not see step1 or step2's local values
				_, ok := stp.State().Local().Get("my-key")
				assert.False(t, ok, "step3 should not see step1/step2 local state")
				return SuccessReport(stp)
			})

		wb.Steps(step1, step2, step3)

		wf, err := wb.Build()
		require.NoError(t, err)

		report := wf.Execute(context.Background())

		assert.True(t, report.IsSuccess())
		assert.Equal(t, "step2-value", step2LocalValue)
	})

	t.Run("steps can read from shared global state", func(t *testing.T) {
		wb := NewWorkflowBuilder().WithId("test-workflow")

		// Step 1: writes to global state
		step1 := NewStepBuilder().WithId("step1").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				stp.State().Global().Set("shared-config", "production")
				stp.State().Global().Set("counter", 1)
				return SuccessReport(stp)
			})

		// Step 2: reads from global and updates it
		step2 := NewStepBuilder().WithId("step2").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				config := stp.State().Global().String("shared-config")
				assert.Equal(t, "production", config)

				counter := stp.State().Global().Int("counter")
				assert.Equal(t, 1, counter)

				stp.State().Global().Set("counter", counter+1)
				return SuccessReport(stp)
			})

		// Step 3: sees updated global state
		step3 := NewStepBuilder().WithId("step3").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				config := stp.State().Global().String("shared-config")
				assert.Equal(t, "production", config)

				counter := stp.State().Global().Int("counter")
				assert.Equal(t, 2, counter)
				return SuccessReport(stp)
			})

		wb.Steps(step1, step2, step3)

		wf, err := wb.Build()
		require.NoError(t, err)

		report := wf.Execute(context.Background())

		assert.True(t, report.IsSuccess())

		// Verify workflow state has global values
		assert.Equal(t, "production", wf.State().Global().String("shared-config"))
		assert.Equal(t, 2, wf.State().Global().Int("counter"))
	})
}

func TestNamespacedStateBag_Merge(t *testing.T) {
	t.Run("merge local namespaces", func(t *testing.T) {
		ns1 := NewNamespacedStateBag(nil, nil)
		ns1.Local().Set("key1", "value1")
		ns1.Local().Set("shared-key", "original")

		ns2 := NewNamespacedStateBag(nil, nil)
		ns2.Local().Set("key2", "value2")
		ns2.Local().Set("shared-key", "merged")

		result, err := ns1.Merge(ns2)
		require.NoError(t, err)

		// Verify it returns itself
		assert.Same(t, ns1, result)

		// Verify local namespace has both keys
		assert.Equal(t, "value1", ns1.Local().String("key1"))
		assert.Equal(t, "value2", ns1.Local().String("key2"))
		assert.Equal(t, "merged", ns1.Local().String("shared-key"), "shared key should be overwritten")
	})

	t.Run("merge global namespaces", func(t *testing.T) {
		ns1 := NewNamespacedStateBag(nil, nil)
		ns1.Global().Set("config", "production")
		ns1.Global().Set("counter", 1)

		ns2 := NewNamespacedStateBag(nil, nil)
		ns2.Global().Set("counter", 5)
		ns2.Global().Set("new-setting", "enabled")

		ns1.Merge(ns2)

		// Verify global namespace has merged values
		assert.Equal(t, "production", ns1.Global().String("config"))
		assert.Equal(t, 5, ns1.Global().Int("counter"), "counter should be updated")
		assert.Equal(t, "enabled", ns1.Global().String("new-setting"))
	})

	t.Run("merge custom namespaces - add new", func(t *testing.T) {
		ns1 := NewNamespacedStateBag(nil, nil)
		ns1.WithNamespace("ns1").Set("key", "value1")

		ns2 := NewNamespacedStateBag(nil, nil)
		ns2.WithNamespace("ns2").Set("key", "value2")

		ns1.Merge(ns2)

		// Verify both custom namespaces exist
		assert.Equal(t, "value1", ns1.WithNamespace("ns1").String("key"))
		assert.Equal(t, "value2", ns1.WithNamespace("ns2").String("key"))
	})

	t.Run("merge custom namespaces - merge existing", func(t *testing.T) {
		ns1 := NewNamespacedStateBag(nil, nil)
		ns1.WithNamespace("shared").Set("key1", "value1")
		ns1.WithNamespace("shared").Set("shared-key", "original")

		ns2 := NewNamespacedStateBag(nil, nil)
		ns2.WithNamespace("shared").Set("key2", "value2")
		ns2.WithNamespace("shared").Set("shared-key", "merged")

		ns1.Merge(ns2)

		// Verify custom namespace has merged values
		sharedNs := ns1.WithNamespace("shared")
		assert.Equal(t, "value1", sharedNs.String("key1"))
		assert.Equal(t, "value2", sharedNs.String("key2"))
		assert.Equal(t, "merged", sharedNs.String("shared-key"))
	})

	t.Run("merge all namespaces together", func(t *testing.T) {
		ns1 := NewNamespacedStateBag(nil, nil)
		ns1.Local().Set("local1", "l1")
		ns1.Global().Set("global1", "g1")
		ns1.WithNamespace("custom1").Set("c1", "v1")
		ns1.WithNamespace("shared").Set("s1", "original")

		ns2 := NewNamespacedStateBag(nil, nil)
		ns2.Local().Set("local2", "l2")
		ns2.Global().Set("global2", "g2")
		ns2.WithNamespace("custom2").Set("c2", "v2")
		ns2.WithNamespace("shared").Set("s2", "merged")

		ns1.Merge(ns2)

		// Verify local namespace
		assert.Equal(t, "l1", ns1.Local().String("local1"))
		assert.Equal(t, "l2", ns1.Local().String("local2"))

		// Verify global namespace
		assert.Equal(t, "g1", ns1.Global().String("global1"))
		assert.Equal(t, "g2", ns1.Global().String("global2"))

		// Verify custom namespaces
		assert.Equal(t, "v1", ns1.WithNamespace("custom1").String("c1"))
		assert.Equal(t, "v2", ns1.WithNamespace("custom2").String("c2"))
		assert.Equal(t, "original", ns1.WithNamespace("shared").String("s1"))
		assert.Equal(t, "merged", ns1.WithNamespace("shared").String("s2"))
	})

	t.Run("merge with nil returns self", func(t *testing.T) {
		ns := NewNamespacedStateBag(nil, nil)
		ns.Local().Set("key", "value")

		result, err := ns.Merge(nil)
		require.NoError(t, err)

		assert.Same(t, ns, result)
		assert.Equal(t, "value", ns.Local().String("key"), "original data should be unchanged")
	})

	t.Run("merge does not affect source", func(t *testing.T) {
		ns1 := NewNamespacedStateBag(nil, nil)
		ns1.Local().Set("key", "value1")

		ns2 := NewNamespacedStateBag(nil, nil)
		ns2.Local().Set("key", "value2")

		ns1.Merge(ns2)

		// Modify ns1 after merge
		ns1.Local().Set("key", "modified")

		// Verify ns2 is not affected
		assert.Equal(t, "value2", ns2.Local().String("key"))
	})

	t.Run("merge custom namespaces are cloned not referenced", func(t *testing.T) {
		ns1 := NewNamespacedStateBag(nil, nil)

		ns2 := NewNamespacedStateBag(nil, nil)
		ns2.WithNamespace("custom").Set("key", "original")

		ns1.Merge(ns2)

		// Modify ns2's custom namespace after merge
		ns2.WithNamespace("custom").Set("key", "modified")

		// Verify ns1's custom namespace is not affected
		assert.Equal(t, "original", ns1.WithNamespace("custom").String("key"))
	})
}

func TestNamespacedStateBag_RealWorldScenario_BindMount(t *testing.T) {
	t.Run("multiple bind mount steps with local state isolation", func(t *testing.T) {
		wb := NewWorkflowBuilder().WithId("bind-mount-workflow")

		// Shared global configuration
		wb.WithState(NewNamespacedStateBag(nil, nil))

		// Setup global config that all steps can read
		setupConfig := NewStepBuilder().WithId("setup-config").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				stp.State().Global().Set("env", "production")
				stp.State().Global().Set("sandbox-dir", "/var/sandbox")
				return SuccessReport(stp)
			})

		// Each bind mount step uses local state for its own bind mount
		type BindMount struct {
			Source string
			Target string
		}

		var bindMount1, bindMount2 BindMount
		var env1, env2 string
		var executeMount1, executeMount2 BindMount

		setupBindMount1 := NewStepBuilder().WithId("setup-bind-mount-1").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				// Read shared config from global
				env1 = stp.State().Global().String("env")
				sandboxDir := stp.State().Global().String("sandbox-dir")

				// Store bind mount in local state (isolated)
				bindMount := BindMount{
					Source: sandboxDir + "/app1",
					Target: "/mnt/app1",
				}
				executeMount1 = bindMount
				stp.State().Local().Set("bind-mount", bindMount)

				return SuccessReport(stp)
			}).
			WithRollback(func(ctx context.Context, stp Step) *Report {
				// Read from local state
				if val, ok := stp.State().Local().Get("bind-mount"); ok {
					bindMount1 = val.(BindMount)
				}
				return SuccessReport(stp)
			})

		setupBindMount2 := NewStepBuilder().WithId("setup-bind-mount-2").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				// Read same shared config from global
				env2 = stp.State().Global().String("env")
				sandboxDir := stp.State().Global().String("sandbox-dir")

				// Store different bind mount in local state (isolated)
				bindMount := BindMount{
					Source: sandboxDir + "/app2",
					Target: "/mnt/app2",
				}
				executeMount2 = bindMount
				stp.State().Local().Set("bind-mount", bindMount)

				return SuccessReport(stp)
			}).
			WithRollback(func(ctx context.Context, stp Step) *Report {
				// Read from local state
				if val, ok := stp.State().Local().Get("bind-mount"); ok {
					bindMount2 = val.(BindMount)
				}
				return SuccessReport(stp)
			})

		wb.Steps(setupConfig, setupBindMount1, setupBindMount2)
		wf, err := wb.Build()
		require.NoError(t, err)

		// Execute workflow
		report := wf.Execute(context.Background())
		assert.True(t, report.IsSuccess())

		// Verify both steps read same global config
		assert.Equal(t, "production", env1)
		assert.Equal(t, "production", env2)

		// Verify execution set the values correctly
		assert.Equal(t, "/var/sandbox/app1", executeMount1.Source)
		assert.Equal(t, "/mnt/app1", executeMount1.Target)
		assert.Equal(t, "/var/sandbox/app2", executeMount2.Source)
		assert.Equal(t, "/mnt/app2", executeMount2.Target)

		// Trigger rollback to verify local state isolation
		rollbackReport := wf.Rollback(context.Background())
		assert.True(t, rollbackReport.IsSuccess())

		// Verify each step retrieved its own bind mount during rollback
		assert.Equal(t, "/var/sandbox/app1", bindMount1.Source)
		assert.Equal(t, "/mnt/app1", bindMount1.Target)
		assert.Equal(t, "/var/sandbox/app2", bindMount2.Source)
		assert.Equal(t, "/mnt/app2", bindMount2.Target)
	})
}

func TestNamespacedStateBag_CustomNamespacePerStep(t *testing.T) {
	t.Run("steps use custom namespaces for isolation", func(t *testing.T) {
		wb := NewWorkflowBuilder().WithId("custom-namespace-workflow")

		type Config struct {
			Host string
			Port int
		}

		var config1, config2 Config

		// Step 1 uses custom namespace "database-1"
		step1 := NewStepBuilder().WithId("setup-db1").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				cfg := Config{Host: "db1.example.com", Port: 5432}
				stp.State().WithNamespace("database-1").Set("config", cfg)
				return SuccessReport(stp)
			}).
			WithRollback(func(ctx context.Context, stp Step) *Report {
				if val, ok := stp.State().WithNamespace("database-1").Get("config"); ok {
					config1 = val.(Config)
				}
				return SuccessReport(stp)
			})

		// Step 2 uses custom namespace "database-2"
		step2 := NewStepBuilder().WithId("setup-db2").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				cfg := Config{Host: "db2.example.com", Port: 3306}
				stp.State().WithNamespace("database-2").Set("config", cfg)
				return SuccessReport(stp)
			}).
			WithRollback(func(ctx context.Context, stp Step) *Report {
				if val, ok := stp.State().WithNamespace("database-2").Get("config"); ok {
					config2 = val.(Config)
				}
				return SuccessReport(stp)
			})

		wb.Steps(step1, step2)
		wf, err := wb.Build()
		require.NoError(t, err)

		// Execute and rollback
		executeReport := wf.Execute(context.Background())
		assert.True(t, executeReport.IsSuccess())

		rollbackReport := wf.Rollback(context.Background())
		assert.True(t, rollbackReport.IsSuccess())

		// Verify each step accessed its own namespace
		assert.Equal(t, "db1.example.com", config1.Host)
		assert.Equal(t, 5432, config1.Port)
		assert.Equal(t, "db2.example.com", config2.Host)
		assert.Equal(t, 3306, config2.Port)
	})
}

func TestNamespacedStateBag_NoUnintentionalOverwrite(t *testing.T) {
	t.Run("steps cannot overwrite each other's local state", func(t *testing.T) {
		wb := NewWorkflowBuilder().WithId("no-overwrite-workflow")

		var step1Value, step2Value, step3Value string

		// All steps use same key "KEY1" but in local namespace
		step1 := NewStepBuilder().WithId("step1").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				stp.State().Local().Set("KEY1", "Value-Step1")
				step1Value = stp.State().Local().String("KEY1")
				return SuccessReport(stp)
			})

		step2 := NewStepBuilder().WithId("step2").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				stp.State().Local().Set("KEY1", "Value-Step2")
				step2Value = stp.State().Local().String("KEY1")

				// Verify step2 doesn't see step1's value
				assert.NotEqual(t, "Value-Step1", step2Value)
				return SuccessReport(stp)
			})

		step3 := NewStepBuilder().WithId("step3").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				stp.State().Local().Set("KEY1", "Value-Step3")
				step3Value = stp.State().Local().String("KEY1")

				// Verify step3 doesn't see step1 or step2's values
				assert.NotEqual(t, "Value-Step1", step3Value)
				assert.NotEqual(t, "Value-Step2", step3Value)
				return SuccessReport(stp)
			})

		wb.Steps(step1, step2, step3)
		wf, err := wb.Build()
		require.NoError(t, err)

		report := wf.Execute(context.Background())
		assert.True(t, report.IsSuccess())

		// Verify each step had its own isolated value
		assert.Equal(t, "Value-Step1", step1Value)
		assert.Equal(t, "Value-Step2", step2Value)
		assert.Equal(t, "Value-Step3", step3Value)
	})

	t.Run("global state can be intentionally shared and overwritten", func(t *testing.T) {
		wb := NewWorkflowBuilder().WithId("shared-global-workflow")

		var step1GlobalValue, step2GlobalValue, step3GlobalValue string

		// All steps use same key "KEY1" in global namespace
		step1 := NewStepBuilder().WithId("step1").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				stp.State().Global().Set("KEY1", "Initial-Value")
				step1GlobalValue = stp.State().Global().String("KEY1")
				return SuccessReport(stp)
			})

		step2 := NewStepBuilder().WithId("step2").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				// Should see step1's value
				step2GlobalValue = stp.State().Global().String("KEY1")

				// Update global state
				stp.State().Global().Set("KEY1", "Updated-By-Step2")
				return SuccessReport(stp)
			})

		step3 := NewStepBuilder().WithId("step3").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				// Should see step2's updated value
				step3GlobalValue = stp.State().Global().String("KEY1")
				return SuccessReport(stp)
			})

		wb.Steps(step1, step2, step3)
		wf, err := wb.Build()
		require.NoError(t, err)

		report := wf.Execute(context.Background())
		assert.True(t, report.IsSuccess())

		// Verify global state was shared and updated
		assert.Equal(t, "Initial-Value", step1GlobalValue)
		assert.Equal(t, "Initial-Value", step2GlobalValue)
		assert.Equal(t, "Updated-By-Step2", step3GlobalValue)
	})
}

func TestNamespacedStateBag_LocalVsGlobalClearSemantics(t *testing.T) {
	t.Run("same key in local and global returns different values", func(t *testing.T) {
		wb := NewWorkflowBuilder().WithId("clear-semantics-workflow")

		step := NewStepBuilder().WithId("test-step").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				// Set same key in different namespaces
				stp.State().Local().Set("KEY1", "Local-Value")
				stp.State().Global().Set("KEY1", "Global-Value")

				// Verify they return different values
				localValue := stp.State().Local().String("KEY1")
				globalValue := stp.State().Global().String("KEY1")

				assert.Equal(t, "Local-Value", localValue)
				assert.Equal(t, "Global-Value", globalValue)
				assert.NotEqual(t, localValue, globalValue)

				return SuccessReport(stp)
			})

		wb.Steps(step)
		wf, err := wb.Build()
		require.NoError(t, err)

		report := wf.Execute(context.Background())
		assert.True(t, report.IsSuccess())
	})

	t.Run("custom namespace independent from local and global", func(t *testing.T) {
		wb := NewWorkflowBuilder().WithId("custom-independent-workflow")

		step := NewStepBuilder().WithId("test-step").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				// Set same key in all three namespace types
				stp.State().Local().Set("KEY1", "Local-Value")
				stp.State().Global().Set("KEY1", "Global-Value")
				stp.State().WithNamespace("custom").Set("KEY1", "Custom-Value")

				// Verify they're all independent
				localValue := stp.State().Local().String("KEY1")
				globalValue := stp.State().Global().String("KEY1")
				customValue := stp.State().WithNamespace("custom").String("KEY1")

				assert.Equal(t, "Local-Value", localValue)
				assert.Equal(t, "Global-Value", globalValue)
				assert.Equal(t, "Custom-Value", customValue)

				return SuccessReport(stp)
			})

		wb.Steps(step)
		wf, err := wb.Build()
		require.NoError(t, err)

		report := wf.Execute(context.Background())
		assert.True(t, report.IsSuccess())
	})
}

func TestNamespacedStateBag_RollbackStateIsolation(t *testing.T) {
	t.Run("rollback receives correct local state snapshot", func(t *testing.T) {
		wb := NewWorkflowBuilder().WithId("rollback-isolation-workflow").WithExecutionMode(RollbackOnError)

		var rollbackValue string
		var rollbackExecuted bool

		step1 := NewStepBuilder().WithId("step1").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				stp.State().Local().Set("data", "step1-data")
				return SuccessReport(stp)
			}).
			WithRollback(func(ctx context.Context, stp Step) *Report {
				// Should see step1's local data during rollback
				rollbackExecuted = true
				rollbackValue = stp.State().Local().String("data")
				return SuccessReport(stp)
			})

		step2 := NewStepBuilder().WithId("step2").
			WithExecute(func(ctx context.Context, stp Step) *Report {
				stp.State().Local().Set("data", "step2-data")
				// Simulate failure to trigger rollback
				return FailureReport(stp, WithError(IllegalArgument.New("simulated failure")))
			})

		wb.Steps(step1, step2)
		wf, err := wb.Build()
		require.NoError(t, err)

		// Execute will fail on step2, triggering rollback
		report := wf.Execute(context.Background())
		assert.False(t, report.IsSuccess())

		// Verify rollback was executed
		assert.True(t, rollbackExecuted, "step1 rollback should have been executed")

		// Verify step1's rollback saw its own local data, not step2's
		assert.Equal(t, "step1-data", rollbackValue)
	})
}

// Create a mock NamespacedStateBag implementation
type mockNamespacedStateBag struct{}

func (m *mockNamespacedStateBag) Local() StateBag                    { return &SyncStateBag{} }
func (m *mockNamespacedStateBag) Global() StateBag                   { return &SyncStateBag{} }
func (m *mockNamespacedStateBag) WithNamespace(name string) StateBag { return &SyncStateBag{} }
func (m *mockNamespacedStateBag) Clone() (NamespacedStateBag, error) { return m, nil }
func (m *mockNamespacedStateBag) Merge(other NamespacedStateBag) (NamespacedStateBag, error) {
	return m, nil
}

func TestSyncNamespacedStateBag_Merge_TypeCheck(t *testing.T) {
	ns := NewNamespacedStateBag(nil, nil)
	mock := &mockNamespacedStateBag{}

	// ✅ Should return error for non-*SyncNamespacedStateBag
	_, err := ns.Merge(mock)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be *SyncNamespacedStateBag")
}
