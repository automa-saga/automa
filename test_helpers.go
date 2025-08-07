package automa

func newSimpleTask(id string) *Task {
	return &Task{
		ID: id,
		Run: func(ctx *Context) error {
			return nil
		},
		Rollback: func(ctx *Context) error {
			return nil
		},
	}
}
