package bge

import "slices"

type runtimeTarget uint8

const (
	linuxAMD64GPU runtimeTarget = 1 << iota
	linuxARM64GPU
	linuxAMD64CPU
	linuxARM64CPU
	darwinRelease

	cudaReleases      = linuxAMD64GPU | linuxARM64GPU
	linuxReleases     = cudaReleases | linuxAMD64CPU | linuxARM64CPU
	allReleaseTargets = linuxReleases | darwinRelease
)

func auditTarget(osName, arch string, tags []string) runtimeTarget {
	if osName == "darwin" {
		return darwinRelease
	}
	if osName != "linux" {
		return 0
	}
	cuda := slices.Contains(tags, "cuda")
	switch arch {
	case "amd64":
		if cuda {
			return linuxAMD64GPU
		}
		return linuxAMD64CPU
	case "arm64":
		if cuda {
			return linuxARM64GPU
		}
		return linuxARM64CPU
	}
	return 0
}

// Each entry limits the occurrences of a reviewed source site by pin and
// release target. The platform reads are separate from the #70 violations.
var reviewedDependencySites = map[runtimePin]map[runtimeSite]runtimeAllowance{
	{"github.com/JohannesKaufmann/html-to-markdown/v2", "v2.5.2"}: {
		{"github.com/JohannesKaufmann/html-to-markdown/v2/converter/ctx.go", "GetDomain", "fmt.Println", ""}:             {allReleaseTargets, 1, "conversion diagnostic through redirected stdout"},
		{"github.com/JohannesKaufmann/html-to-markdown/v2/converter/ctx.go", "UpdateState (closure)", "fmt.Println", ""}: {allReleaseTargets, 1, "conversion diagnostic through redirected stdout"},
		{"github.com/JohannesKaufmann/html-to-markdown/v2/converter/ctx.go", "assembleAbsoluteURL", "fmt.Println", ""}:   {allReleaseTargets, 1, "conversion diagnostic through redirected stdout"},
	},
	{"github.com/ebitengine/purego", "v0.10.1"}: {
		{"github.com/ebitengine/purego/internal/fakecgo/go_linux.go", "_cgo_sys_thread_start", "builtin.print", ""}:   {cudaReleases, 1, "fatal purego thread-bootstrap diagnostic before abort"},
		{"github.com/ebitengine/purego/internal/fakecgo/go_linux.go", "_cgo_sys_thread_start", "builtin.println", ""}: {cudaReleases, 1, "fatal purego thread-bootstrap diagnostic before abort"},
		{"github.com/ebitengine/purego/internal/fakecgo/go_linux.go", "x_cgo_init", "builtin.println", ""}:            {cudaReleases, 1, "fatal purego thread-bootstrap diagnostic before abort"},
		{"github.com/ebitengine/purego/internal/fakecgo/go_util.go", "x_cgo_thread_start", "builtin.println", ""}:     {cudaReleases, 1, "fatal purego thread-bootstrap diagnostic before abort"},
	},
	{"github.com/eitamring/gocudrv", "v0.3.2"}: {
		{"github.com/eitamring/gocudrv/internal/hostcb/hostcb.go", "Invoke (closure)", "os.Stderr", ""}: {cudaReleases, 1, "host callback diagnostic through the current Go stderr"},
	},
	{"github.com/spf13/pflag", "v1.1.0"}: {
		{"github.com/spf13/pflag/flag.go", "(*FlagSet).Output", "os.Stderr", ""}: {allReleaseTargets, 1, "dormant default stderr when pbrain supplies the flag writer"},
	},
	{"go.yaml.in/yaml/v3", "v3.0.6"}: {
		{"go.yaml.in/yaml/v3/scannerc.go", "trace", "fmt.Println", ""}:           {allReleaseTargets, 1, "dormant YAML parser trace helper"},
		{"go.yaml.in/yaml/v3/scannerc.go", "trace (closure)", "fmt.Println", ""}: {allReleaseTargets, 1, "dormant YAML parser trace helper"},
	},
	{"golang.org/x/sys", "v0.50.0"}: {
		{"golang.org/x/sys/cpu/cpu.go", "processOptions", "builtin.print", ""}:        {linuxAMD64GPU | linuxAMD64CPU | darwinRelease, 5, "approved Go CPU feature policy and diagnostics at startup"},
		{"golang.org/x/sys/cpu/cpu.go", "processOptions", "os.Getenv", "\"GODEBUG\""}: {linuxAMD64GPU | linuxAMD64CPU | darwinRelease, 1, "approved Go CPU feature policy and diagnostics at startup"},
		{"golang.org/x/sys/unix/env_unix.go", "Environ", "syscall.Environ", ""}:       {allReleaseTargets, 1, "dormant syscall wrapper with no linked caller"},
		{"golang.org/x/sys/unix/env_unix.go", "Getenv", "syscall.Getenv", "key"}:      {allReleaseTargets, 1, "dormant syscall wrapper with no linked caller"},
	},
	{"modernc.org/libc", "v1.80.1"}: {
		{"modernc.org/libc/libc.go", "Start", "os.Getwd", ""}:                          {darwinRelease, 1, "dormant C executable startup"},
		{"modernc.org/libc/libc_darwin.go", "Xsystem", "os/exec.Command", ""}:          {darwinRelease, 1, "dormant C shell with PATH lookup and inherited environment"},
		{"modernc.org/libc/libc_darwin.go", "Xgetlogin_r", "os/user.Current", ""}:      {darwinRelease, 1, "dormant C login lookup outside the in-memory SQLite path"},
		{"modernc.org/libc/libc_musl.go", "Xsystem", "os/exec.Command", ""}:            {linuxReleases, 1, "dormant C shell with PATH lookup and inherited environment"},
		{"modernc.org/libc/libc_unix.go", "Xtmpfile", "os.CreateTemp", ""}:             {darwinRelease, 1, "dormant file-backed temporary storage outside the in-memory SQLite path"},
		{"modernc.org/libc/builtin.go", "X__builtin_unreachable", "os.Stderr", ""}:     {linuxReleases, 2, "fatal C diagnostic through the current Go stderr"},
		{"modernc.org/libc/etc.go", "todo", "os.Stdout", ""}:                           {darwinRelease, 2, "C trace and unimplemented-operation diagnostics through the current Go stdout"},
		{"modernc.org/libc/etc.go", "trc", "os.Stdout", ""}:                            {darwinRelease, 2, "C trace and unimplemented-operation diagnostics through the current Go stdout"},
		{"modernc.org/libc/etc_musl.go", "trc", "os.Stderr", ""}:                       {linuxReleases, 2, "C trace through the current Go stderr"},
		{"modernc.org/libc/libc.go", "Environ", "os.Environ", ""}:                      {darwinRelease, 1, "dormant Darwin C environment accessor outside the in-memory SQLite path"},
		{"modernc.org/libc/libc.go", "EnvironP", "os.Environ", ""}:                     {darwinRelease, 1, "dormant Darwin C environment accessor outside the in-memory SQLite path"},
		{"modernc.org/libc/libc.go", "Start", "os.Environ", ""}:                        {darwinRelease, 1, "dormant C executable startup"},
		{"modernc.org/libc/libc.go", "Start", "os.Getenv", "\"LIBC_MEMGRIND_START\""}:  {darwinRelease, 1, "dormant C executable startup"},
		{"modernc.org/libc/libc.go", "X__assert_fail", "os.Stderr", ""}:                {darwinRelease, 3, "C assertion output through the current Go stderr, with one dormant stack dump"},
		{"modernc.org/libc/libc.go", "X__builtin_unreachable", "os.Stderr", ""}:        {darwinRelease, 2, "C output through the current Go stream"},
		{"modernc.org/libc/libc.go", "Xputs", "fmt.Printf", ""}:                        {darwinRelease, 1, "C output through the current Go stream"},
		{"modernc.org/libc/libc.go", "exit", "os.Stderr", ""}:                          {darwinRelease, 1, "dormant coverage and allocator audit output"},
		{"modernc.org/libc/libc.go", "exit", "os.Stdout", ""}:                          {darwinRelease, 2, "dormant coverage and allocator audit output"},
		{"modernc.org/libc/libc.go", "setEnviron", "os.Environ", ""}:                   {darwinRelease, 1, "dormant Darwin C environment initialization"},
		{"modernc.org/libc/libc.go", "write", "os.Stdout", ""}:                         {darwinRelease, 1, "C output through the current Go stream"},
		{"modernc.org/libc/libc_darwin.go", "Xgetlogin_r", "os.Getenv", "\"LOGNAME\""}: {darwinRelease, 1, "dormant shell or login operation outside the in-memory SQLite path"},
		{"modernc.org/libc/libc_darwin.go", "Xgetlogin_r", "os.Getenv", "\"USER\""}:    {darwinRelease, 1, "dormant shell or login operation outside the in-memory SQLite path"},
		{"modernc.org/libc/libc_darwin.go", "Xperror", "os.Stderr", ""}:                {darwinRelease, 2, "C perror through the current Go stderr"},
		{"modernc.org/libc/libc_darwin.go", "Xsystem", "os.Stderr", ""}:                {darwinRelease, 1, "dormant shell or login operation outside the in-memory SQLite path"},
		{"modernc.org/libc/libc_darwin.go", "Xsystem", "os.Stdout", ""}:                {darwinRelease, 1, "dormant shell or login operation outside the in-memory SQLite path"},
		{"modernc.org/libc/libc_musl.go", "Environ (closure)", "os.Environ", ""}:       {linuxReleases, 1, "dormant alternative to the first SQLite TLS snapshot"},
		{"modernc.org/libc/libc_musl.go", "EnvironP (closure)", "os.Environ", ""}:      {linuxReleases, 1, "dormant alternative to the first SQLite TLS snapshot"},
		{"modernc.org/libc/libc_musl.go", "NewTLS (closure)", "os.Environ", ""}:        {linuxReleases, 1, "approved Linux SQLite C ABI environment snapshot at startup"},
		{"modernc.org/libc/libc_musl.go", "Xsystem", "os.Stderr", ""}:                  {linuxReleases, 1, "dormant C shell execution"},
		{"modernc.org/libc/libc_musl.go", "Xsystem", "os.Stdout", ""}:                  {linuxReleases, 1, "dormant C shell execution"},
		{"modernc.org/libc/watch.go", "Watch", "os.Stderr", ""}:                        {allReleaseTargets, 12, "dormant tracing for registered watches"},
	},
	{"modernc.org/memory", "v1.12.1"}: {
		{"modernc.org/memory/memory.go", "(*Allocator).Trim (closure)", "os.Stderr", ""}:           {allReleaseTargets, 1, "dormant allocator tracing with the trace constant disabled"},
		{"modernc.org/memory/memory.go", "(*Allocator).UintptrCalloc (closure)", "os.Stderr", ""}:  {allReleaseTargets, 1, "dormant allocator tracing with the trace constant disabled"},
		{"modernc.org/memory/memory.go", "(*Allocator).UintptrFree (closure)", "os.Stderr", ""}:    {allReleaseTargets, 1, "dormant allocator tracing with the trace constant disabled"},
		{"modernc.org/memory/memory.go", "(*Allocator).UintptrMalloc (closure)", "os.Stderr", ""}:  {allReleaseTargets, 1, "dormant allocator tracing with the trace constant disabled"},
		{"modernc.org/memory/memory.go", "(*Allocator).UintptrRealloc (closure)", "os.Stderr", ""}: {allReleaseTargets, 1, "dormant allocator tracing with the trace constant disabled"},
		{"modernc.org/memory/memory.go", "UintptrUsableSize (closure)", "os.Stderr", ""}:           {allReleaseTargets, 1, "dormant allocator tracing with the trace constant disabled"},
	},
}
