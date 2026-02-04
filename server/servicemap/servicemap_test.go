package servicemap

import "testing"

func TestBuildServiceMap(t *testing.T) {
	conns := []Connection{
		{SourceApp: "frontend", DestApp: "backend", Protocol: "http", RequestRate: 100},
		{SourceApp: "backend", DestApp: "postgres", Protocol: "tcp", RequestRate: 50},
		{SourceApp: "cron", DestApp: "backend", Protocol: "http", RequestRate: 1},
	}

	sm := BuildServiceMap(conns)

	if len(sm.Applications) != 4 {
		t.Fatalf("expected 4 applications, got %d", len(sm.Applications))
	}

	if !sm.HasDependency("frontend", "backend") {
		t.Fatalf("expected frontend -> backend dependency")
	}

	if !sm.HasDependency("backend", "postgres") {
		t.Fatalf("expected backend -> postgres dependency")
	}

	if sm.HasDependency("postgres", "backend") {
		t.Fatalf("did not expect postgres -> backend dependency")
	}
}
