package agent_test

import "github.com/WilsonSousajr/omatty/internal/hooks"

func hooksPayload(event string) hooks.Payload {
	return hooks.Payload{SessionID: "s", HookEventName: event}
}
