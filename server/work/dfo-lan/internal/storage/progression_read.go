package storage

import "context"

// RunMonsterExperience reads the committed monster event total for one run.
func (s *Store) RunMonsterExperience(ctx context.Context, account, character int64, run string) (uint64, error) {
	var total uint64
	err := s.DB.QueryRow(ctx, `SELECT COALESCE(SUM((e.outcome->>'gain')::bigint),0)::bigint FROM character_events e JOIN characters c ON c.id=e.character_id WHERE c.account_id=$1 AND c.id=$2 AND e.event_key LIKE $3`, account, character, "monster:"+run+":%").Scan(&total)
	return total, err
}

// RunFatigueLedger returns charged fatigue and the first-loaded-room count.
func (s *Store) RunFatigueLedger(ctx context.Context, account, character int64, run string) (int64, int64, error) {
	var charged, rooms int64
	err := s.DB.QueryRow(ctx, `SELECT COALESCE(SUM(f.cost),0)::bigint,COUNT(*)::bigint FROM character_fatigue_rooms f JOIN characters c ON c.id=f.character_id WHERE c.account_id=$1 AND c.id=$2 AND f.run_id=$3`, account, character, run).Scan(&charged, &rooms)
	return charged, rooms, err
}
