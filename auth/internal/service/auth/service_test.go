package auth

import (
	"sync"
	"testing"
)

type rotateRepo struct {
	mu    sync.Mutex
	valid map[string]int64
}

func (r *rotateRepo) RotateRefresh(oldJTI, newJTI string, userID int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	uid, ok := r.valid[oldJTI]
	if !ok || uid != userID {
		return false, nil
	}
	delete(r.valid, oldJTI)
	r.valid[newJTI] = userID
	return true, nil
}

func TestRotateRefreshRace(t *testing.T) {
	repo := &rotateRepo{valid: map[string]int64{"old": 42}}
	var ok1, ok2 bool
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		ok1, _ = repo.RotateRefresh("old", "new1", 42)
	}()
	go func() {
		defer wg.Done()
		ok2, _ = repo.RotateRefresh("old", "new2", 42)
	}()
	wg.Wait()
	if ok1 == ok2 {
		t.Fatalf("expected exactly one winner, got ok1=%v ok2=%v", ok1, ok2)
	}
	if len(repo.valid) != 1 {
		t.Fatalf("expected one active refresh, got %d", len(repo.valid))
	}
}
