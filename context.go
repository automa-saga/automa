package automa

import (
	"context"
	"github.com/rs/zerolog"
	"sync"
)

// Context wraps the standard context.Context and adds thread-safe key-value storage.
// It enables chaining custom context operations and maintains compatibility with the context package.
type Context struct {
	context.Context                // Embedded standard context for cancellation, deadlines, etc.
	mu              sync.Mutex     // Mutex to protect concurrent access to values.
	values          map[string]any // Custom key-value pairs for workflow state.
}

// SetValue stores a key-value pair in the context.
// Thread-safe: locks the context during mutation.
// Returns the context for chaining.
func (c *Context) SetValue(key string, value any) *Context {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.values == nil {
		c.values = make(map[string]any)
	}
	c.values[key] = value
	return c
}

// GetValue retrieves a value by key from the context's custom storage.
// Returns nil if the key does not exist.
func (c *Context) GetValue(key string) any {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.values == nil {
		return nil
	}
	return c.values[key]
}

// Value implements context.Context's Value method.
// Only supports string keys for custom storage.
// Returns nil for non-string keys or missing values.
func (c *Context) Value(key any) any {
	ks, ok := key.(string)
	if !ok {
		return nil
	}
	return c.GetValue(ks)
}

// WithCancel creates a new Context derived from the current one, with a cancel function.
// Copies the custom values to the new context for isolation.
// Returns the new context and its cancel function.
func (c *Context) WithCancel() (*Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(c.Context)

	// Copy custom values for the new context.
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
// If parentCtx is nil, context.Background() is used.
// Initializes an empty custom values map.
func NewContext(parentCtx context.Context) *Context {
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	return &Context{Context: parentCtx, values: make(map[string]any)}
}

func (c *Context) getPrevResult() *Result {
	prevResult := c.GetValue(KeyPrevResult)
	if prevResult == nil {
		return nil
	}
	return prevResult.(*Result)
}
func (c *Context) setPrevResult(result *Result) *Context {
	return c.SetValue(KeyPrevResult, result)
}

func (c *Context) getLogger() zerolog.Logger {
	logger := c.GetValue(KeyLogger)
	if logger == nil {
		return nolog // Return a no-op logger if none is set
	}
	return logger.(zerolog.Logger)
}
