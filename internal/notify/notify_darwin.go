//go:build darwin

package notify

// New returns the notifier for this platform.
//
//	model.SetNotifier(notify.New())
func New() Notifier { return Osascript{} }
