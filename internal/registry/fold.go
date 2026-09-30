package registry

// SetCollapsed records whether a project is folded in the sidebar (#505). It
// is the whole of what `ctrl+o tab` persists: the fold is a view preference,
// but one the operator expects to survive a restart.
//
//	err := registry.SetCollapsed(store, "omatty", true)
func SetCollapsed(s StateStore, project string, collapsed bool) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	if _, err := findProject(&st, project); err != nil {
		return err
	}
	for i := range st.Projects {
		if st.Projects[i].Name == project {
			st.Projects[i].Collapsed = collapsed
		}
	}
	return s.Save(st)
}
