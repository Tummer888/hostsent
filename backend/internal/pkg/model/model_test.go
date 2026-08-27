package model

import (
	"encoding/json"
	"testing"
)

func TestStandardProductJSONSerialization(t *testing.T) {
	p := StandardProduct{
		ID:           1,
		ProviderType: "mofangyun",
		UpstreamID:   "up_001",
		Name:         "标准云主机",
		Specs: StandardProductSpec{
			CPU:       4,
			Memory:    8192,
			Disk:      80,
			DiskType:  "ssd",
			Bandwidth: 10,
			OS:        "ubuntu",
			Region:    "cn-east-1",
			Zone:      "cn-east-1a",
		},
		CostPrice: 30.5,
		SalePrice: 50.0,
		Status:    "active",
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded StandardProduct
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.ProviderType != "mofangyun" {
		t.Errorf("ProviderType mismatch: got %s", decoded.ProviderType)
	}
	if decoded.Specs.CPU != 4 {
		t.Errorf("Specs.CPU mismatch: got %d", decoded.Specs.CPU)
	}
}

func TestStandardProductSpecRoundTrip(t *testing.T) {
	spec := StandardProductSpec{
		CPU:    2,
		Memory: 4096,
		Disk:   50,
		Extra:  map[string]interface{}{"gpu": false},
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded StandardProductSpec
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.Extra["gpu"] != false {
		t.Errorf("Extra.gpu mismatch: got %v", decoded.Extra["gpu"])
	}
	if decoded.DiskType != "" {
		t.Errorf("expected empty DiskType, got %q", decoded.DiskType)
	}
}

func TestStandardInstanceStatus(t *testing.T) {
	if InstanceStatusRunning != "running" {
		t.Errorf("unexpected running status: %s", InstanceStatusRunning)
	}
	if InstanceStatusCreating != "creating" {
		t.Errorf("unexpected creating status: %s", InstanceStatusCreating)
	}
}

func TestCreateInstanceRequestSerialization(t *testing.T) {
	req := CreateInstanceRequest{
		ProviderType: "mofangyun",
		ProductID:    7,
		Name:         "web-01",
		Password:     "secret",
		Region:       "cn-east-1",
		Count:        1,
		BillingMode:  "hourly",
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded CreateInstanceRequest
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.ProductID != 7 {
		t.Errorf("ProductID mismatch: got %d", decoded.ProductID)
	}
	if decoded.BillingMode != "hourly" {
		t.Errorf("BillingMode mismatch: got %s", decoded.BillingMode)
	}
}
