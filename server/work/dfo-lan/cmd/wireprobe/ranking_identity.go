package main

import "dfolan/internal/ranking"

func (client *gameConnection) sendRankingUserKey() error {
	if client.worldState == nil || client.worldState.store == nil || client.worldState.account <= 0 {
		return nil
	}
	body, err := ranking.UserKey(client.worldState.account)
	if err != nil {
		return err
	}
	if err := client.output.send(0, 2631, body); err != nil {
		return err
	}
	// Never record the opaque identifier in logs.
	client.event(map[string]any{"kind": "ranking_identity_sent", "bytes": len(body), "attempt": "2/3"})
	return nil
}
