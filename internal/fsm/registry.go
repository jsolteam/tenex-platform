package fsm

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// TransitionRule описывает допустимый переход из одного состояния в другое.
type TransitionRule struct {
	// From — исходное состояние. Idle означает переход из начального состояния.
	From State
	// To — целевое состояние.
	To State
	// Description — человекочитаемое описание перехода для документации и отладки.
	Description string
}

// Registry — реестр всех состояний и допустимых переходов.
// Позволяет валидировать переходы до их выполнения и генерировать документацию.
//
// Каждый файл scenarios/*.go вызывает registry.MustRegister() в init()
// для декларативного описания своего сценария.
type Registry struct {
	mu          sync.RWMutex
	transitions map[State]map[State]TransitionRule
	states      map[State]string // state -> description
}

// globalRegistry — единственный экземпляр реестра для всего приложения.
var globalRegistry = &Registry{
	transitions: make(map[State]map[State]TransitionRule),
	states:      make(map[State]string),
}

// GlobalRegistry возвращает глобальный реестр.
func GlobalRegistry() *Registry { return globalRegistry }

// RegisterState регистрирует состояние с описанием.
func (r *Registry) RegisterState(state State, description string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.states[state]; exists {
		panic(fmt.Sprintf("fsm: состояние %q уже зарегистрировано", state))
	}
	r.states[state] = description
}

// RegisterTransition регистрирует допустимый переход между двумя состояниями.
func (r *Registry) RegisterTransition(rule TransitionRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.transitions[rule.From] == nil {
		r.transitions[rule.From] = make(map[State]TransitionRule)
	}
	key := rule.To
	if _, exists := r.transitions[rule.From][key]; exists {
		panic(fmt.Sprintf("fsm: переход %s → %s уже зарегистрирован", rule.From, rule.To))
	}
	r.transitions[rule.From][key] = rule
}

// MustRegister регистрирует сразу несколько переходов. Используется в scenarios/*.go.
func (r *Registry) MustRegister(rules ...TransitionRule) {
	for _, rule := range rules {
		r.RegisterTransition(rule)
	}
}

// IsAllowed проверяет допустимость перехода из from в to.
// Если переходы для from не зарегистрированы — переход считается запрещённым.
// Переход в Idle всегда разрешён (Reset доступен из любого состояния).
func (r *Registry) IsAllowed(from, to State) bool {
	if to == Idle {
		return true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	targets, ok := r.transitions[from]
	if !ok {
		return false
	}
	_, allowed := targets[to]
	return allowed
}

// AllowedFrom возвращает все допустимые целевые состояния из данного состояния.
func (r *Registry) AllowedFrom(from State) []State {
	r.mu.RLock()
	defer r.mu.RUnlock()
	targets := r.transitions[from]
	result := make([]State, 0, len(targets))
	for to := range targets {
		result = append(result, to)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// Describe возвращает описание состояния или пустую строку если не зарегистрировано.
func (r *Registry) Describe(state State) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.states[state]
}

// States возвращает все зарегистрированные состояния в алфавитном порядке.
func (r *Registry) States() []State {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]State, 0, len(r.states))
	for s := range r.states {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// Dot генерирует граф переходов в формате Graphviz DOT для визуализации.
//	dot -Tpng -o fsm.png <<< $(go run ./tools/fsm_graph)
func (r *Registry) Dot() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("digraph FSM {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=rounded];\n")
	sb.WriteString(`  "(idle)" [shape=circle, style=filled, fillcolor=lightgreen];` + "\n")

	for from, targets := range r.transitions {
		fromLabel := from.String()
		for to, rule := range targets {
			toLabel := to.String()
			label := rule.Description
			sb.WriteString(fmt.Sprintf("  %q -> %q [label=%q];\n", fromLabel, toLabel, label))
		}
	}
	// Переходы в Idle из всех состояний (Reset)
	for s := range r.states {
		if s != Idle {
			sb.WriteString(fmt.Sprintf("  %q -> %q [label=\"reset\", style=dashed];\n", s.String(), Idle.String()))
		}
	}
	sb.WriteString("}\n")
	return sb.String()
}
