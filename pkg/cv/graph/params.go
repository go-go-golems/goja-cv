package graph

import (
	"fmt"
	"math"
	"sort"
	"sync"
)

// ParamDef is a declared param. Values are float64 (slider), bool (toggle)
// or string (choice).
type ParamDef struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"` // "slider" | "toggle" | "choice"
	Min     float64  `json:"min,omitempty"`
	Max     float64  `json:"max,omitempty"`
	Step    float64  `json:"step,omitempty"`
	Options []string `json:"options,omitempty"`
	Value   any      `json:"value"`
	Default any      `json:"default"` // the value the agent declared
	Label   string   `json:"label,omitempty"`
	Unit    string   `json:"unit,omitempty"`
	Touched bool     `json:"touched"` // moved by Set (the person) since declaration
}

// ParamStore holds the current param values. A param node's evaluation
// signature includes its current value, so moving a param invalidates
// exactly the nodes downstream of it.
type ParamStore struct {
	mu   sync.RWMutex
	defs map[string]*ParamDef
}

// NewParamStore returns an empty store.
func NewParamStore() *ParamStore { return &ParamStore{defs: map[string]*ParamDef{}} }

func (d *ParamDef) check(v any) (any, error) {
	switch d.Type {
	case "slider":
		f, ok := normalize(v).(float64)
		if !ok || math.IsNaN(f) {
			return nil, Errorf(EArgType, "param %q is a slider; its value must be a number, got %s.", d.Name, describeRaw(normalize(v)))
		}
		if f < d.Min || f > d.Max {
			return nil, Errorf(EArgRange, "param %q = %s is outside [%s, %s].", d.Name, fmtNum(f), fmtNum(d.Min), fmtNum(d.Max)).
				WithHint("Out-of-range values are errors, not clamps.")
		}
		return f, nil
	case "toggle":
		b, ok := v.(bool)
		if !ok {
			return nil, Errorf(EArgType, "param %q is a toggle; its value must be true or false.", d.Name)
		}
		return b, nil
	case "choice":
		s, ok := v.(string)
		if !ok || !contains(d.Options, s) {
			return nil, Errorf(EArgRange, "param %q must be one of %s.", d.Name, quoteList(d.Options)).WithValid(d.Options)
		}
		return s, nil
	}
	return nil, Errorf(EInternal, "param %q has unknown type %q", d.Name, d.Type)
}

func (d *ParamDef) sameShape(o *ParamDef) bool {
	if d.Type != o.Type || d.Min != o.Min || d.Max != o.Max || len(d.Options) != len(o.Options) {
		return false
	}
	for i := range d.Options {
		if d.Options[i] != o.Options[i] {
			return false
		}
	}
	return true
}

// Declare adds a param or returns the existing one. Re-declaring with the
// same shape keeps the current value (the person's value wins); a different
// range is E_PARAM_REDECLARE unless reset is true.
func (s *ParamStore) Declare(def ParamDef, reset bool) (ParamDef, error) {
	if def.Name == "" {
		return ParamDef{}, Errorf(EArgType, "a param needs a name.")
	}
	if def.Type == "slider" && !(def.Min < def.Max) {
		return ParamDef{}, Errorf(EArgRange, "param %q: min (%s) must be below max (%s).", def.Name, fmtNum(def.Min), fmtNum(def.Max))
	}
	if def.Type == "choice" && len(def.Options) == 0 {
		return ParamDef{}, Errorf(EArgType, "param %q: a choice needs options.", def.Name)
	}
	if def.Value == nil {
		switch def.Type {
		case "slider":
			def.Value = def.Min
		case "toggle":
			def.Value = false
		case "choice":
			def.Value = def.Options[0]
		}
	}
	v, err := def.check(def.Value)
	if err != nil {
		return ParamDef{}, err
	}
	def.Value, def.Default, def.Touched = v, v, false
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.defs[def.Name]; ok && !reset {
		if !old.sameShape(&def) {
			return ParamDef{}, Errorf(EParamRedeclare, "param %q is already declared with a different range.", def.Name).
				WithFix(fmt.Sprintf("pass { reset: true } to redeclare %q", def.Name))
		}
		if !old.Touched {
			old.Value, old.Default = def.Value, def.Default
		}
		old.Label, old.Unit, old.Step = def.Label, def.Unit, def.Step
		return *old, nil
	}
	d := def
	s.defs[def.Name] = &d
	return d, nil
}

// Set moves a param (as the person would). The value is range-checked.
func (s *ParamStore) Set(name string, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.defs[name]
	if !ok {
		return Errorf(EArgType, "no param named %q.", name).WithValid(s.namesLocked())
	}
	c, err := d.check(v)
	if err != nil {
		return err
	}
	d.Value, d.Touched = c, true
	return nil
}

// Get returns a copy of the param definition.
func (s *ParamStore) Get(name string) (ParamDef, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.defs[name]
	if !ok {
		return ParamDef{}, false
	}
	return *d, true
}

// Snapshot returns the current values of all params.
func (s *ParamStore) Snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]any, len(s.defs))
	for k, d := range s.defs {
		out[k] = d.Value
	}
	return out
}

// List returns all params sorted by name.
func (s *ParamStore) List() []ParamDef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ParamDef, 0, len(s.defs))
	for _, d := range s.defs {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *ParamStore) namesLocked() []string {
	out := make([]string, 0, len(s.defs))
	for k := range s.defs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
