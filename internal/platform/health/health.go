package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Check func(ctx context.Context) error

type Node struct {
	Name  string
	Check Check
	Deps  []*Node
}

type Result struct {
	Name     string            `json:"name"`
	Healthy  bool              `json:"healthy"`
	Error    string            `json:"error,omitempty"`
	Duration time.Duration     `json:"duration"`
	Deps     map[string]Result `json:"deps,omitempty"`
}

// Add appends a named dependency node to n and returns the created node.
func (n *Node) Add(name string, check Check) *Node {
	child := &Node{Name: name, Check: check}
	n.Deps = append(n.Deps, child)
	return child
}

func Evaluate(ctx context.Context, n *Node) Result {
	start := time.Now()
	res := Result{
		Name: n.Name,
		Deps: map[string]Result{},
	}
	if n.Check != nil {
		if err := n.Check(ctx); err != nil {
			res.Healthy = false
			res.Error = err.Error()
			res.Duration = time.Since(start)
			return res
		}
	}
	for _, d := range n.Deps {
		dr := Evaluate(ctx, d)
		res.Deps[dr.Name] = dr
		if !dr.Healthy {
			res.Healthy = false
		}
	}
	// If we made it here and didn't fail checks, it's healthy.
	if res.Error == "" && res.Healthy == false && len(res.Deps) == 0 {
		res.Healthy = true
	}
	if res.Error == "" && (len(res.Deps) > 0) {
		// healthy if all deps healthy
		all := true
		for _, dr := range res.Deps {
			if !dr.Healthy {
				all = false
				break
			}
		}
		res.Healthy = all
	}
	res.Duration = time.Since(start)
	return res
}

// Handler returns an http.Handler that evaluates the dependency graph.
// If serving() is provided and returns false, the handler returns 503 immediately.
func Handler(root *Node, serving func() bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serving != nil && !serving() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("NOT_SERVING"))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		out := Evaluate(ctx, root)
		if !out.Healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		w.Header().Set("content-type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	})
}

// Livez is a simple liveness handler.
func Livez() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}
