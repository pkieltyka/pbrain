//go:build linux && amd64 && vulkan

package gemma2

import "testing"

func TestVulkanCompiled(t *testing.T) {
	if !vulkanCompiled {
		t.Fatal("a build with the vulkan tag does not report the Vulkan backend")
	}
}
