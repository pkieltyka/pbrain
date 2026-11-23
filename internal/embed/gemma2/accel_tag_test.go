//go:build !linux || !amd64 || !vulkan

package gemma2

import "testing"

func TestVulkanNotCompiled(t *testing.T) {
	if vulkanCompiled {
		t.Fatal("a build without the vulkan tag reports the Vulkan backend")
	}
}
