package status

// HookPayload is the slice of a hook's stdin that status needs.
//
// Source is SessionStart's reason ("startup", "resume", "clear", "compact").
// OmattySession is never read from stdin: Report stamps it from SessionEnv,
// so a payload cannot claim a pane it was not launched in (#316).
type HookPayload struct {
	SessionID        string `json:"session_id"`
	HookEventName    string `json:"hook_event_name"`
	NotificationType string `json:"notification_type,omitempty"`
	ToolName         string `json:"tool_name,omitempty"`
	Source           string `json:"source,omitempty"`
	OmattySession    string `json:"omatty_session,omitempty"`
}
