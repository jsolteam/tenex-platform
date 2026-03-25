// Package fsm реализует конечный автомат (FSM) для управления пользовательскими
//
// Добавление нового сценария:
//  1. Создать файл internal/fsm/scenarios/myscenario.go
//  2. Объявить константы состояний через конструктор NewState("scenario", "step")
//  3. Зарегистрировать допустимые переходы через registry.MustRegister(...)
//  4. Реализовать хэндлеры в internal/bot/handler/myscenario/
package fsm

import (
	"encoding/json"
	"fmt"
	"sync"
)

// State — имя FSM-состояния пользователя.
// Формат: "сценарий:шаг", например "add_medicine:name".
// Специальное значение "" означает простой (idle) — пользователь не в сценарии.
type State string

// Idle — начальное состояние: пользователь не в активном сценарии.
const Idle State = ""

// IsIdle возвращает true если состояние является пустым (начальным).
func (s State) IsIdle() bool { return s == Idle }

// String возвращает строковое представление состояния.
func (s State) String() string {
	if s == Idle {
		return "(idle)"
	}
	return string(s)
}

// NewState конструирует State из имени сценария и шага.
// Гарантирует единообразный формат "scenario:step" во всём проекте.
func NewState(scenario, step string) State {
	if scenario == "" {
		panic("fsm.NewState: scenario не может быть пустым")
	}
	if step == "" {
		panic("fsm.NewState: step не может быть пустым")
	}
	return State(fmt.Sprintf("%s:%s", scenario, step))
}

// StateData хранит промежуточные данные сценария.
// Накапливается по мере прохождения шагов — каждый шаг добавляет своё поле.
//
// Пример жизненного цикла при добавлении лекарства:
//
//	шаг name:      data.Set("name", "Аспирин")
//	шаг photo:     data.Set("photo_key", "s3/media/abc.jpg")
//	шаг schedule:  data.Set("schedule_type", "daily")
//	шаг times:     data.Set("times", []string{"08:00", "20:00"})
//	финализация:   все поля читаются и создаётся запись в БД
type StateData struct {
	mu     sync.RWMutex
	fields map[string]interface{}
}

func NewStateData() StateData {
	return StateData{fields: make(map[string]interface{})}
}

func (d *StateData) Set(key string, value interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.fields == nil {
		d.fields = make(map[string]interface{})
	}
	d.fields[key] = value
}

func (d *StateData) Get(key string) interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.fields[key]
}

func (d *StateData) GetString(key string) string {
	v, _ := d.Get(key).(string)
	return v
}

func (d *StateData) GetInt64(key string) int64 {
	v := d.Get(key)
	switch val := v.(type) {
	case int64:
		return val
	case float64:
		return int64(val)
	case int:
		return int64(val)
	}
	return 0
}

func (d *StateData) GetBool(key string) bool {
	v, _ := d.Get(key).(bool)
	return v
}

func (d *StateData) GetStrings(key string) []string {
	v := d.Get(key)
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func (d *StateData) Has(key string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.fields[key]
	return ok
}

func (d *StateData) Delete(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.fields, key)
}

// Clone возвращает глубокую копию StateData.
// Используется при передаче данных между горутинами.
func (d *StateData) Clone() StateData {
	d.mu.RLock()
	defer d.mu.RUnlock()
	cp := StateData{fields: make(map[string]interface{}, len(d.fields))}
	for k, v := range d.fields {
		cp.fields[k] = v
	}
	return cp
}

func (d *StateData) MarshalJSON() ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return json.Marshal(d.fields)
}

func (d *StateData) UnmarshalJSON(b []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.fields = make(map[string]interface{})
	return json.Unmarshal(b, &d.fields)
}

// FSMState — полное состояние пользовательской сессии в FSM.
// Хранится в Redis и передаётся между методами Machine.
type FSMState struct {
	State State     `json:"state"`
	Data  StateData `json:"data"`
}

// NewFSMState создаёт FSMState с указанным состоянием и пустыми данными.
func NewFSMState(state State) *FSMState {
	return &FSMState{
		State: state,
		Data:  NewStateData(),
	}
}

// NewIdleFSMState создаёт FSMState в начальном состоянии Idle.
func NewIdleFSMState() *FSMState {
	return NewFSMState(Idle)
}
