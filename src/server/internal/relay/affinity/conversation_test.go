package affinity

import (
	"testing"
	"time"
)

func TestConversationAffinity_BindAndGet(t *testing.T) {
	ca := NewConversationAffinity(time.Hour, 3)

	ca.Bind("conv1", "ch1", "org1")

	ch, ok := ca.GetChannel("conv1")
	if !ok || ch != "ch1" {
		t.Fatalf("expected ch1, got %s (ok=%v)", ch, ok)
	}
}

func TestConversationAffinity_TTL(t *testing.T) {
	ca := NewConversationAffinity(100*time.Millisecond, 3)

	ca.Bind("conv1", "ch1", "org1")

	_, ok := ca.GetChannel("conv1")
	if !ok {
		t.Fatal("should find channel before TTL")
	}

	time.Sleep(150 * time.Millisecond)

	_, ok = ca.GetChannel("conv1")
	if ok {
		t.Fatal("should NOT find channel after TTL")
	}
}

func TestConversationAffinity_Failover(t *testing.T) {
	ca := NewConversationAffinity(time.Hour, 3)

	ca.Bind("conv1", "ch1", "org1")

	// Failover to ch2
	ok := ca.Failover("conv1", "ch2")
	if !ok {
		t.Fatal("failover should succeed")
	}

	ch, _ := ca.GetChannel("conv1")
	if ch != "ch2" {
		t.Fatalf("expected ch2 after failover, got %s", ch)
	}
}

func TestConversationAffinity_MaxFailovers(t *testing.T) {
	ca := NewConversationAffinity(time.Hour, 2)

	ca.Bind("conv1", "ch1", "org1")

	ca.Failover("conv1", "ch2")
	ca.Failover("conv1", "ch3")

	// Third failover should remove the mapping
	ok := ca.Failover("conv1", "ch4")
	if ok {
		t.Fatal("failover should fail after max attempts")
	}

	_, exists := ca.GetChannel("conv1")
	if exists {
		t.Fatal("mapping should be removed after max failovers")
	}
}

func TestConversationAffinity_Remove(t *testing.T) {
	ca := NewConversationAffinity(time.Hour, 3)

	ca.Bind("conv1", "ch1", "org1")
	ca.Remove("conv1")

	_, ok := ca.GetChannel("conv1")
	if ok {
		t.Fatal("should not find removed mapping")
	}
}

func TestConversationAffinity_RemoveByOrg(t *testing.T) {
	ca := NewConversationAffinity(time.Hour, 3)

	ca.Bind("conv1", "ch1", "org1")
	ca.Bind("conv2", "ch2", "org1")
	ca.Bind("conv3", "ch3", "org2")

	ca.RemoveByOrg("org1")

	_, ok := ca.GetChannel("conv1")
	if ok {
		t.Fatal("org1 conv1 should be removed")
	}
	_, ok = ca.GetChannel("conv2")
	if ok {
		t.Fatal("org1 conv2 should be removed")
	}

	ch, ok := ca.GetChannel("conv3")
	if !ok || ch != "ch3" {
		t.Fatal("org2 conv3 should still exist")
	}
}

func TestConversationAffinity_Cleanup(t *testing.T) {
	ca := NewConversationAffinity(100*time.Millisecond, 3)

	ca.Bind("conv1", "ch1", "org1")
	ca.Bind("conv2", "ch2", "org2")

	time.Sleep(150 * time.Millisecond)

	removed := ca.Cleanup()
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}

	stats := ca.Stats()
	if stats.TotalMappings != 0 {
		t.Fatalf("expected 0 mappings after cleanup, got %d", stats.TotalMappings)
	}
}

func TestConversationAffinity_IsCacheShareable(t *testing.T) {
	ca := NewConversationAffinity(time.Hour, 3)

	ca.Bind("conv1", "ch1", "org1")
	ca.Bind("conv2", "ch2", "org1")
	ca.Bind("conv3", "ch3", "org2")

	// Same org -> shareable
	if !ca.IsCacheShareable("conv1", "conv2") {
		t.Fatal("same org should be shareable")
	}

	// Different org -> not shareable
	if ca.IsCacheShareable("conv1", "conv3") {
		t.Fatal("different org should not be shareable")
	}
}

func TestConversationAffinity_Stats(t *testing.T) {
	ca := NewConversationAffinity(time.Hour, 3)

	ca.Bind("conv1", "ch1", "org1")
	ca.Bind("conv2", "ch2", "org1")
	ca.Failover("conv2", "ch3")

	stats := ca.Stats()
	if stats.TotalMappings != 2 {
		t.Fatalf("expected 2 mappings, got %d", stats.TotalMappings)
	}
	if stats.FailoverMappings != 1 {
		t.Fatalf("expected 1 failover mapping, got %d", stats.FailoverMappings)
	}
	if stats.OrgCount != 1 {
		t.Fatalf("expected 1 org, got %d", stats.OrgCount)
	}
}
