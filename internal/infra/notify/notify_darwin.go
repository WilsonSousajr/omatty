//go:build darwin

package notify

// New returns the notifier for this platform.
//
//	ui.NewModel(ui.Deps{Notifier: notify.New(), /* ... */})
func New() Notifier { return Osascript{} }
