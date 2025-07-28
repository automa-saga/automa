package automa

import (
	"context"
	"sync"
)

// Context wraps context.Context and allows chaining custom context operations.
// It provides a thread-safe way to set and get key-value pairs in the context.
// The Context struct embeds context.Context to maintain compatibility with the standard context package.
type Context struct {
	context.Context
	mu     sync.Mutex
	values map[string]any
}

// SetValue sets a key-value pair in the context.
// It locks the context to ensure thread safety when accessing the values map.
// If the values map is nil, it initializes it before setting the value.
func (c *Context) SetValue(key string, value any) *Context {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure the values map is initialized
	if c.values == nil {
		c.values = make(map[string]any)
	}

	c.values[key] = value

	return c
}

// GetValue retrieves a value from the context by its key.
func (c *Context) GetValue(key string) any {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.values == nil {
		return nil
	}

	if v, ok := c.values[key]; ok {
		return v
	}

	return nil
}

// Value retrieves a value from the context using the standard context.Value method.
// It overrides the default implementation to use the custom GetValue method and support a string key only.
// This is useful for compatibility with the context package's Value method.
// If the key is not a string, it returns nil.
func (c *Context) Value(key any) any {
	ks, ok := key.(string)
	if !ok {
		return nil
	}

	return c.GetValue(ks)
}

// WithCancel creates a new Context that is derived from the current context with a cancel function.
// This allows the new context to be cancelled independently of the parent context.
// It also copies the values map to ensure that the new context has its own copy of the values.
// The returned cancel function can be used to cancel the new context.
func (c *Context) WithCancel() (*Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(c.Context)

	// make copy of the values map
	valuesCopy := make(map[string]any)
	c.mu.Lock()
	if c.values != nil {
		for k, v := range c.values {
			valuesCopy[k] = v
		}
	}
	c.mu.Unlock()

	return &Context{
		Context: ctx,
		values:  valuesCopy,
	}, cancel
}

// NewContext creates a new automa.Context from a parent context.
// If parentCtx is nil, it defaults to context.Background().
func NewContext(parentCtx context.Context) *Context {
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	return &Context{Context: parentCtx, values: make(map[string]any)}
}

// getPrevSuccess retrieves the previous success event from the context.
func (c *Context) getPrevSuccess() *Success {
	prevSuccess := c.GetValue(KeyPrevSuccess)
	if prevSuccess == nil {
		return nil
	}
	return prevSuccess.(*Success)
}

// getPrevFailure retrieves the previous failure event from the context.
func (c *Context) getPrevFailure() *Failure {
	prevFailure := c.GetValue(KeyPrevFailure)
	if prevFailure == nil {
		return nil
	}
	return prevFailure.(*Failure)
}
