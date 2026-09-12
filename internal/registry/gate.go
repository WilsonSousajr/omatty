package registry

import (
	"github.com/WilsonSousajr/omatty/internal/gate"
)

// SetGate records the verification commands a project is checked by, replacing
// whatever it had. It is what the confirm picker and `omatty gate --set` both
// call, so the two cannot disagree about what "setting a gate" means.
//
//	err := registry.SetGate(store, "omatty", gate.Detect(root))
//
// Setting a gate is deliberately a separate act from proposing one: detection
// only ever proposes, and nothing runs a gate the operator has not confirmed.
func SetGate(s *Store, project string, steps []gate.Step) error {
	return editGate(s, project, steps)
}

// ClearGate forgets a project's gate, returning it to "not configured yet"
// rather than "configured as nothing" - the distinction the nil in
// Project.Gate carries.
func ClearGate(s *Store, project string) error {
	return editGate(s, project, nil)
}

// editGate is the load-find-write the two commands share.
func editGate(s *Store, project string, steps []gate.Step) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	if _, err := findProject(&st, project); err != nil {
		return err
	}
	for i := range st.Projects {
		if st.Projects[i].Name == project {
			st.Projects[i].Gate = steps
		}
	}
	return s.Save(st)
}
