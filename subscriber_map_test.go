package actioncable

import "testing"

func TestRunAndClose(t *testing.T) {
	sm := &SubscriberMap{}
	sm.Run()

	if sm.done == nil || sm.sending == nil {
		t.Error("channels are not initialized.")
	}

	if sm.broadcastConcurrentNum != 100 {
		t.Error("broadcastConcurrentNum is unset.")
	}

	sm.Stop()

	_, open := <-sm.done

	if open {
		t.Error("done channel is still open.")
	}
}
