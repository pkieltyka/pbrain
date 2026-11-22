package cli

import (
	"fmt"
	"math"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/render"
)

const bytesPerMiB = 1 << 20

var deviceSelector = regexp.MustCompile(`^[a-z]+(:[a-zA-Z0-9_.-]+)?$`)

func syncRuntimeOptions(env Env, in *invocation) (embed.OpenOptions, error) {
	selector := env.Getenv("PBRAIN_DEVICE")
	if in.has("device") {
		selector = in.string("device")
	}
	if (in.has("device") || selector != "") && !deviceSelector.MatchString(selector) {
		return embed.OpenOptions{}, &errs.Error{Code: "invalid_config",
			Message: "invalid device selector; use auto, cpu, or a device from pbrain devices", Exit: 1}
	}
	opt := embed.OpenOptions{Device: selector}
	if raw := env.Getenv("PBRAIN_GPU_MEMORY"); raw != "" {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || n == 0 || n > math.MaxInt64/bytesPerMiB {
			return embed.OpenOptions{}, &errs.Error{Code: "invalid_config",
				Message: "PBRAIN_GPU_MEMORY must be a whole number of MiB of at least 1", Exit: 1}
		}
		opt.MemoryBudget = int64(n * bytesPerMiB)
	}
	return opt, nil
}

func deviceNotice(info embed.RunInfo, model string) (errs.Notice, bool) {
	if info.Device != "" && info.Device != "cpu" {
		message := info.Device
		if info.Name != "" {
			message += " (" + info.Name + ")"
		}
		if info.Budget > 0 {
			message += fmt.Sprintf(", %d MiB a batch", info.Budget/bytesPerMiB)
		}
		return errs.Notice{Code: "embedding_device", Message: message}, true
	}

	var reason string
	switch info.Fallback {
	case embed.FallbackAsked:
		reason = "the CPU was asked for"
	case embed.FallbackNoSupport:
		reason = "this pbrain has no GPU support for " + model
	case embed.FallbackNoDevice:
		reason = "no usable GPU was found"
	case embed.FallbackFailed:
		reason = "the GPU could not be used"
	case embed.FallbackMemory:
		reason = "the GPU has too little free memory"
	default:
		return errs.Notice{}, false
	}
	return errs.Notice{Code: "cpu_fallback", Message: "embedding on the CPU, which is slow: " + reason}, true
}

func runDevices(env Env, in *invocation) outcome {
	reg := env.Models
	if reg == nil {
		reg = embed.NewRegistry()
	}
	var text strings.Builder
	var rows []render.Object
	for _, d := range reg.Devices() {
		row := render.Object{"id": d.ID, "backend": d.Backend}
		parts := []string{d.ID}
		if d.ID == "cpu" {
			threads := runtime.GOMAXPROCS(0)
			row["threads"] = threads
			parts = append(parts, counted(threads, "thread", "threads"))
		} else {
			if d.Name != "" {
				row["name"] = d.Name
				parts = append(parts, d.Name)
			}
			if d.Persistent != "" {
				row["persistent"] = d.Persistent
			}
			if d.MemoryKind != "" {
				row["memory_kind"] = d.MemoryKind
			}
			var memory string
			if d.TotalBytes > 0 {
				row["total_bytes"] = d.TotalBytes
				memory = fmt.Sprintf("%d MiB", d.TotalBytes/bytesPerMiB)
			}
			if d.Memory == "live" || d.Memory == "estimated" {
				row["memory"] = d.Memory
				row["free_bytes"] = d.FreeBytes
				if memory == "" {
					memory = fmt.Sprintf("%d MiB free", d.FreeBytes/bytesPerMiB)
				} else {
					memory += fmt.Sprintf(", %d free", d.FreeBytes/bytesPerMiB)
				}
			}
			if memory != "" {
				parts = append(parts, memory)
			}
		}
		text.WriteString(strings.Join(parts, "  ") + "\n")
		rows = append(rows, row)
	}
	return textOrJSON(in.json, text.String(), render.Object{"devices": rows})
}
