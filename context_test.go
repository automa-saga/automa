package automa

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewContext(t *testing.T) {
	base := context.Background()
	c := NewContext(base)
	assert.NotNil(t, c)
	assert.Equal(t, base, c.Context)
}

func TestSetValue(t *testing.T) {
	base := context.Background()
	c := NewContext(base)
	key1 := "foo-1"
	val1 := "bar-1"
	key2 := "foo-2"
	val2 := "bar-2"
	c.SetValue(key1, val1)
	assert.Equal(t, val1, c.GetValue(key1))
	assert.Equal(t, val1, c.Value(key1))
	assert.Equal(t, nil, c.GetValue(key2))
	assert.Equal(t, nil, c.Value(key2))

	c.SetValue(key2, val2)
	assert.NotNil(t, c)
	assert.Equal(t, val1, c.Value(key1))
	assert.Equal(t, val1, c.Value(key1))
	assert.Equal(t, val1, c.GetValue(key1))
	assert.Equal(t, val2, c.GetValue(key2))
	assert.Equal(t, nil, c.Value(struct{}{})) // Test with a non-existent and invalid key
}

func TestWithCancel(t *testing.T) {
	base := context.Background()
	c := NewContext(base)
	c.SetValue("foo", "bar")
	c2, cancel := c.WithCancel()
	assert.NotNil(t, c2)
	assert.NotNil(t, cancel)
	assert.NotSame(t, c, c2)

	c2.SetValue("bar", "baz")
	c2.SetValue("foo", "qux")
	assert.Equal(t, "bar", c.Value("foo")) // Original context should not be affected
	assert.Equal(t, "qux", c2.Value("foo"))
	assert.Equal(t, "baz", c2.Value("bar"))

	done := make(chan struct{})
	go func() {
		<-c2.Done()
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("context was not cancelled")
	}
}
