package automa

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type cloneErrStateBag struct{ *SyncStateBag }

func (b *cloneErrStateBag) Clone() (StateBag, error) { return nil, errors.New("clone failed") }

func TestWorkflow_SnapshotJournalFailsWhenGlobalCloneFails(t *testing.T) {
	wf := &workflow{
		id:    "wf",
		state: NewNamespacedStateBag(nil, &cloneErrStateBag{SyncStateBag: &SyncStateBag{}}),
		steps: []Step{&defaultStep{id: "s"}},
	}

	j, err := wf.snapshotJournal()
	require.Error(t, err)
	require.Nil(t, j)
}
