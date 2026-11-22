package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/pkieltyka/pbrain/internal/errs"
)

func TestCLIDevices(t *testing.T) {
	reg := embed.NewRegistry(embed.Entry{Model: embed.Model{Name: "test-device-model"}, Devices: func() []embed.DeviceInfo {
		return []embed.DeviceInfo{
			{ID: "metal:0", Backend: "metal", TotalBytes: 8192 * bytesPerMiB, MemoryKind: "shared", Memory: "unknown"},
			{ID: "cuda:1", Backend: "cuda", FreeBytes: 0, Memory: "live", MemoryKind: "unknown"},
			{ID: "cuda:0", Backend: "cuda", Name: "Test GPU", Persistent: "cuda:GPU-test", Memory: "live", MemoryKind: "dedicated",
				TotalBytes: 16384 * bytesPerMiB, FreeBytes: 4096 * bytesPerMiB},
			{ID: "vulkan:0", Backend: "vulkan", Name: "Unknown memory GPU"},
		}
	}})
	dir := t.TempDir()
	env := Env{Dir: dir, Models: reg}
	threads := runtime.GOMAXPROCS(0)
	cpuLine := fmt.Sprintf("cpu  %s\n", counted(threads, "thread", "threads"))
	want := "cuda:0  Test GPU  16384 MiB, 4096 free\ncuda:1  0 MiB free\nvulkan:0  Unknown memory GPU\nmetal:0  8192 MiB\n" + cpuLine
	if code, out, stderr := invoke(t, env, Features{}, "--brain", "/no/such/brain", "--personal", "devices"); code != 0 || out != want || stderr != "" {
		t.Fatalf("devices = (%d, %q, %q), want %q", code, out, stderr, want)
	}
	code, out, stderr := invoke(t, env, Features{}, "devices", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("JSON devices = (%d, %q, %q)", code, out, stderr)
	}
	var result struct{ Devices []map[string]any }
	oneJSON(t, out, &result)
	if len(result.Devices) != 5 {
		t.Fatalf("JSON devices = %s", out)
	}
	first, zero, unknown, cpu := result.Devices[0], result.Devices[1], result.Devices[2], result.Devices[4]
	if first["id"] != "cuda:0" || first["name"] != "Test GPU" || first["persistent"] != "cuda:GPU-test" ||
		first["free_bytes"] != float64(4096*bytesPerMiB) || first["total_bytes"] != float64(16384*bytesPerMiB) ||
		first["memory"] != "live" || first["memory_kind"] != "dedicated" ||
		zero["id"] != "cuda:1" || zero["free_bytes"] != float64(0) || zero["total_bytes"] != nil ||
		unknown["id"] != "vulkan:0" || unknown["free_bytes"] != nil ||
		cpu["id"] != "cpu" || cpu["backend"] != "cpu" || cpu["threads"] != float64(threads) || len(cpu) != 3 {
		t.Fatalf("JSON devices = %s", out)
	}
	env.Models = nil
	if code, out, stderr := invoke(t, env, Features{}, "devices"); code != 0 || out != cpuLine || stderr != "" {
		t.Fatalf("CPU-only devices = (%d, %q, %q)", code, out, stderr)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("devices changed its working directory: %v %v", entries, err)
	}
}

type deviceEmbedder struct {
	embed.Embedder
	info, after embed.RunInfo
}

func (e *deviceEmbedder) Info() embed.RunInfo { return e.info }

func (e *deviceEmbedder) Embed(ctx context.Context, role embed.Role, texts []string) ([][]float32, error) {
	if role == embed.Document && e.after.Device != "" {
		e.info = e.after
	}
	return e.Embedder.Embed(ctx, role, texts)
}

func useDeviceModel(rb *readBrain, kit *embedtest.Kit, devices func() []embed.DeviceInfo,
	open func(context.Context, *embed.Artifact, embed.OpenOptions) (embed.Embedder, error)) {
	rb.env.Models = embed.NewRegistry(embed.Entry{Model: kit.Models[embedtest.ModelA], Devices: devices, Open: open})
}

func TestSyncDeviceNotices(t *testing.T) {
	device := embed.RunInfo{Device: "cuda:0", Name: "Test GPU", Budget: 1024 * bytesPerMiB}
	cpu := func(f embed.Fallback) embed.RunInfo { return embed.RunInfo{Device: "cpu", Fallback: f} }
	for _, tt := range []struct {
		name         string
		info, after  embed.RunInfo
		notice, more errs.Notice
	}{
		{name: "accelerator", info: device, notice: errs.Notice{Code: "embedding_device", Message: "cuda:0 (Test GPU), 1024 MiB a batch"}},
		{name: "budget unknown", info: embed.RunInfo{Device: "cuda:0"}, notice: errs.Notice{Code: "embedding_device", Message: "cuda:0"}},
		{name: "normal CPU", info: cpu(embed.NoFallback)},
		{name: "CPU asked for", info: cpu(embed.FallbackAsked), notice: errs.Notice{Code: "cpu_fallback", Message: "embedding on the CPU, which is slow: the CPU was asked for"}},
		{name: "unsupported", info: cpu(embed.FallbackNoSupport), notice: errs.Notice{Code: "cpu_fallback", Message: "embedding on the CPU, which is slow: this pbrain has no GPU support for test-model"}},
		{name: "driver absent", info: cpu(embed.FallbackNoDevice), notice: errs.Notice{Code: "cpu_fallback", Message: "embedding on the CPU, which is slow: no usable GPU was found"}},
		{name: "attach failed", info: cpu(embed.FallbackFailed), notice: errs.Notice{Code: "cpu_fallback", Message: "embedding on the CPU, which is slow: the GPU could not be used"}},
		{name: "memory fallback", info: device, after: cpu(embed.FallbackMemory),
			notice: errs.Notice{Code: "embedding_device", Message: "cuda:0 (Test GPU), 1024 MiB a batch"},
			more:   errs.Notice{Code: "cpu_fallback", Message: "embedding on the CPU, which is slow: the GPU has too little free memory"}},
	} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", tt.name, jsonMode), func(t *testing.T) {
				rb, kit := semanticBrain(t)
				rb.put("personal/notes/20261104-000000-mine-m1m1.md", []string{"kind: note", "title: Mine",
					"created: 2026-11-04T00:00:00Z", "author: agent:x"}, "delta mine\n")
				useDeviceModel(rb, kit, nil, func(ctx context.Context, a *embed.Artifact, o embed.OpenOptions) (embed.Embedder, error) {
					base, err := kit.Registry.Open(ctx, a, embed.OpenOptions{})
					info := tt.info
					if o.Device == "" {
						info = cpu(embed.NoFallback)
					}
					return &deviceEmbedder{Embedder: base, info: info, after: tt.after}, err
				})
				args := []string{"sync", "--personal"}
				if jsonMode {
					args = append(args, "--json")
				}
				code, out, stderr := rb.run(args...)
				if code != 0 {
					t.Fatalf("sync failed = (%d, %q, %q)", code, out, stderr)
				}
				assertDeviceNotices(t, jsonMode, stderr, tt.notice, tt.more)
				if jsonMode {
					device := tt.info.Device
					if tt.after.Device != "" {
						device = tt.after.Device
					}
					assertSyncDevices(t, out, device)
				}
				opens := kit.Opens.Load()
				code, out, stderr = rb.run(args...)
				if code != 0 || stderr != "" || kit.Opens.Load() != opens {
					t.Fatalf("no-op sync = (%d, %q, %q), opened %d extra runtimes", code, out, stderr, kit.Opens.Load()-opens)
				}
				if jsonMode {
					assertSyncDevices(t, out, "")
				}
				if code, _, stderr := rb.run("search", "beta material zzz", "--semantic"); code != 0 || stderr != "" {
					t.Fatalf("search status = %d, stderr = %q", code, stderr)
				}
			})
		}
	}
}

func assertDeviceNotices(t *testing.T, jsonMode bool, stderr string, expected ...errs.Notice) {
	t.Helper()
	var want []errs.Notice
	for _, n := range expected {
		if n.Code != "" {
			want = append(want, n)
		}
	}
	var lines []string
	if stderr != "" {
		lines = strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	}
	if len(lines) != len(want) {
		t.Fatalf("notices = %q, want %+v", stderr, want)
	}
	for i, n := range want {
		if jsonMode {
			var row struct{ Notice errs.Notice }
			oneJSON(t, lines[i]+"\n", &row)
			if row.Notice != n {
				t.Fatalf("notice = %+v, want %+v", row.Notice, n)
			}
		} else if lines[i] != n.Code+": "+n.Message {
			t.Fatalf("notice = %q, want %+v", lines[i], n)
		}
	}
}

func assertSyncDevices(t *testing.T, out, device string) {
	t.Helper()
	var result struct{ Sections []map[string]any }
	oneJSON(t, out, &result)
	if len(result.Sections) != 2 {
		t.Fatalf("sync sections = %s", out)
	}
	for _, row := range result.Sections {
		got, present := row["device"]
		if device == "" && present || device != "" && got != device {
			t.Fatalf("section %v: device = %v, want %q", row["section"], got, device)
		}
	}
}

func TestSyncDeviceOptions(t *testing.T) {
	t.Run("flag and environment", func(t *testing.T) {
		rb, kit := semanticBrain(t)
		var opts []embed.OpenOptions
		useDeviceModel(rb, kit, func() []embed.DeviceInfo {
			return []embed.DeviceInfo{{ID: "cuda:0", Backend: "cuda"}}
		}, func(ctx context.Context, a *embed.Artifact, o embed.OpenOptions) (embed.Embedder, error) {
			opts = append(opts, o)
			if o.Device == "cuda:0" {
				return nil, fmt.Errorf("%w: listed GPU failed to attach", embed.ErrAccelerator)
			}
			return kit.Registry.Open(ctx, a, o)
		})
		getenv := rb.env.Getenv
		rb.env.Getenv = func(name string) string {
			switch name {
			case "PBRAIN_DEVICE":
				return "cuda"
			case "PBRAIN_GPU_MEMORY":
				return "24"
			default:
				return getenv(name)
			}
		}
		code, out, stderr := rb.run("sync")
		if code != 1 || out != "" || !strings.HasPrefix(stderr, "io: the model cannot be opened") ||
			len(opts) != 1 || opts[0] != (embed.OpenOptions{Device: "cuda:0", MemoryBudget: 24 * bytesPerMiB}) {
			t.Fatalf("named attach failure = (%d, %q, %q), options %+v", code, out, stderr, opts)
		}
		code, out, stderr = rb.run("sync", "--device", "cpu")
		if code != 0 || stderr != "" || len(opts) != 2 || opts[1].Device != "cpu" || !strings.Contains(out, "2 passages embedded") {
			t.Fatalf("flag override = (%d, %q, %q), options %+v", code, out, stderr, opts)
		}
		code, out, stderr = rb.run("sync", "--device", "nosuch")
		if code != 0 || stderr != "" || len(opts) != 2 || !strings.Contains(out, "0 passages embedded") {
			t.Fatalf("no-op with missing device = (%d, %q, %q), options %+v", code, out, stderr, opts)
		}
	})
	for _, label := range []string{"flag", "environment"} {
		t.Run("CPU-only model ignores "+label, func(t *testing.T) {
			rb, kit := semanticBrain(t)
			args := []string{"sync", "--json"}
			if label == "flag" {
				args = append(args, "--device", "cuda:99")
			} else {
				getenv := rb.env.Getenv
				rb.env.Getenv = func(name string) string {
					if name == "PBRAIN_DEVICE" {
						return "cuda:99"
					}
					return getenv(name)
				}
			}
			code, out, stderr := rb.run(args...)
			if code != 0 || stderr != "" || kit.Opens.Load() != 1 || !strings.Contains(out, `"device":"cpu"`) {
				t.Fatalf("CPU-only sync = (%d, %q, %q), opens = %d", code, out, stderr, kit.Opens.Load())
			}
		})
	}
	for _, tt := range []struct {
		name, value string
		args        []string
		want        string
	}{
		{name: "PBRAIN_DEVICE", value: "CUDA:0", want: "invalid_config: invalid device selector"},
		{name: "PBRAIN_GPU_MEMORY", value: "1.5", want: "invalid_config: PBRAIN_GPU_MEMORY"},
		{name: "PBRAIN_GPU_MEMORY", value: "9223372036854775807", want: "invalid_config: PBRAIN_GPU_MEMORY"},
		{args: []string{"--device="}, want: "invalid_config: invalid device selector"},
		{args: []string{"--status", "--device", "cpu"}, want: "usage: --device cannot be used with --status"},
	} {
		rb, kit := semanticBrain(t)
		getenv := rb.env.Getenv
		rb.env.Getenv = func(name string) string {
			if name == tt.name {
				return tt.value
			}
			return getenv(name)
		}
		code, out, stderr := rb.run(append([]string{"sync"}, tt.args...)...)
		wantCode := 1
		if strings.HasPrefix(tt.want, "usage:") {
			wantCode = 2
		}
		if code != wantCode || out != "" || !strings.HasPrefix(stderr, tt.want) || kit.Opens.Load() != 0 {
			t.Fatalf("sync %v with %s=%q = (%d, %q, %q)", tt.args, tt.name, tt.value, code, out, stderr)
		}
		if tt.name != "" {
			if _, err := os.Stat(filepath.Join(rb.dir, ".pbrain", "index")); !os.IsNotExist(err) {
				t.Fatal("invalid environment created an index folder")
			}
			if code, _, stderr := rb.run("sync", "--status"); code != 0 || stderr != "" {
				t.Fatalf("status consulted the runtime environment: %d %q", code, stderr)
			}
		}
	}
}
