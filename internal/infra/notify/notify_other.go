//go:build !darwin

package notify

// New returns the notifier for this platform: Silent, until a Linux or
// Windows delivery path exists.
//
//	ui.NewModel(ui.Deps{Notifier: notify.New(), /* ... */})
func New() Notifier { return Silent{} }
