package agent_test

import "github.com/WilsonSousajr/omatty/internal/infra/hooks"

func hooksPayload(event string) hooks.Payload {
	return hooks.Payload{SessionID: "s", HookEventName: event}
}
