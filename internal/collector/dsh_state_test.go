package collector

// 临时验证：模型状态持久化 + 增量路径。验证完成后删除。
import (
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// 内存版 ModelStateStore
type memStore struct{ m map[string]string }

func (s *memStore) GetConfig(key string) (string, error) { return s.m[key], nil }
func (s *memStore) SetConfig(key, value string) error    { s.m[key] = value; return nil }

func TestDSHModelStateIncremental(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "session.jsonl.zstd")

	enc, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))

	// 帧1：header + request/context + 事件（模型 deepseek-v4-flash）
	frame1 := enc.EncodeAll([]byte(`{"type":"session","version":0,"id":"session-test","cwd":"C:\\proj","createdAt":1}
{"type":"request/context","seq":0,"time":1000,"data":{"model":"deepseek-v4-flash"}}
{"type":"assistant/message","seq":1,"time":2000,"data":{"turn":1,"step":1,"usage":{"inputTokens":100,"outputTokens":50}}}
`), nil)
	os.WriteFile(fp, frame1, 0o644)

	store := &memStore{m: map[string]string{}}
	c := NewDSHCollector()
	c.SetStore(store)
	
	// 阶段1：全量（无状态）→ 事件正确 + 模型状态已保存
	recs := c.parseSessionFile(fp, map[string]string{})
	if len(recs) != 1 || recs[0].model != "deepseek-v4-flash" {
		t.Fatalf("stage1: want 1 event model=deepseek-v4-flash, got %+v", recs)
	}
	if store.m[modelStateKey(fp)] != "deepseek-v4-flash" {
		t.Fatalf("stage1: model state not saved, got %q", store.m[modelStateKey(fp)])
	}
	t.Log("stage1 full + state saved ok")

	// 阶段2：追加帧（无 request/context）→ 增量只解新帧，模型继承
	frame2 := enc.EncodeAll([]byte(`{"type":"assistant/message","seq":2,"time":3000,"data":{"turn":2,"step":1,"usage":{"inputTokens":10,"outputTokens":5}}}
`), nil)
	f, _ := os.OpenFile(fp, os.O_APPEND|os.O_WRONLY, 0o644)
	f.Write(frame2)
	f.Close()

	recs2 := c.parseSessionFile(fp, map[string]string{})
	if len(recs2) != 1 || recs2[0].model != "deepseek-v4-flash" || recs2[0].seq != 2 {
		t.Fatalf("stage2: want 1 event seq=2 model=flash, got %+v", recs2)
	}
	t.Log("stage2 incremental model inherited ok")

	// 阶段3：追加含 request/context 的新帧（换模型）→ 模型更新并持久化
	frame3 := enc.EncodeAll([]byte(`{"type":"request/context","seq":3,"time":4000,"data":{"model":"deepseek-v4-pro"}}
{"type":"assistant/message","seq":4,"time":5000,"data":{"turn":3,"step":1,"usage":{"inputTokens":7,"outputTokens":3}}}
`), nil)
	f, _ = os.OpenFile(fp, os.O_APPEND|os.O_WRONLY, 0o644)
	f.Write(frame3)
	f.Close()

	recs3 := c.parseSessionFile(fp, map[string]string{})
	if len(recs3) != 1 || recs3[0].model != "deepseek-v4-pro" {
		t.Fatalf("stage3: want 1 event model=deepseek-v4-pro, got %+v", recs3)
	}
	if store.m[modelStateKey(fp)] != "deepseek-v4-pro" {
		t.Fatalf("stage3: model state not updated, got %q", store.m[modelStateKey(fp)])
	}
	t.Log("stage3 model switch + state update ok")

	// 阶段4：清模型状态 → 回退全量，仍正确
	delete(store.m, modelStateKey(fp))
	frame4 := enc.EncodeAll([]byte(`{"type":"assistant/message","seq":5,"time":6000,"data":{"turn":4,"step":1,"usage":{"inputTokens":1,"outputTokens":1}}}
`), nil)
	f, _ = os.OpenFile(fp, os.O_APPEND|os.O_WRONLY, 0o644)
	f.Write(frame4)
	f.Close()

	recs4 := c.parseSessionFile(fp, map[string]string{})
	if len(recs4) != 1 || recs4[0].model != "deepseek-v4-pro" {
		t.Fatalf("stage4 fallback: want 1 event model=pro, got %+v", recs4)
	}
	t.Log("stage4 no-state fallback ok")
}
