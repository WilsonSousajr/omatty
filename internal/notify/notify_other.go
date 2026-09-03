//go:build !darwin

package notify

// New returns the notifier for this platform: Silent, until a Linux or
// Windows delivery path exists.
//
//	model.SetNotifier(notify.New())
func New() Notifier { return Silent{} }
