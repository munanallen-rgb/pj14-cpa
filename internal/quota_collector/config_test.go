package quotacollector

import "testing"

func TestParseInstancesUsesDefaultAndPerInstanceKeys(t *testing.T) {
	t.Setenv("CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY_CPA2", "specific")

	instances, err := parseInstances("cpa1=http://cpa1:8317,cpa2=http://cpa2:8317", "default")
	if err != nil {
		t.Fatalf("parseInstances: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("instances len = %d, want 2", len(instances))
	}
	if instances[0].ManagementKey != "default" {
		t.Fatalf("cpa1 key = %q, want default", instances[0].ManagementKey)
	}
	if instances[1].ManagementKey != "specific" {
		t.Fatalf("cpa2 key = %q, want specific", instances[1].ManagementKey)
	}
}

func TestParseInstancesRequiresKey(t *testing.T) {
	if _, err := parseInstances("cpa1=http://cpa1:8317", ""); err == nil {
		t.Fatalf("parseInstances without key returned nil error")
	}
}
