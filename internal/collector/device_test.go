package collector

import "testing"

// TestDeviceIdentity 验证采集器设备身份：注入 UUID 后优先使用，缺失时回退本机 hostname。
func TestDeviceIdentity(t *testing.T) {
	d := &DeviceIdentity{}
	if got := d.Device(); got == "" {
		t.Fatal("fallback device should be non-empty hostname")
	}

	d.SetDevice("11111111-2222-4333-8444-555555555555")
	if got := d.Device(); got != "11111111-2222-4333-8444-555555555555" {
		t.Fatalf("injected device not used, got %q", got)
	}

	d.SetDevice("")
	if got := d.Device(); got == "" {
		t.Fatal("empty injection should not clear fallback")
	}
}
