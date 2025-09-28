package sim

import (
	"encoding/json"
	"math/rand"
)

type MetadataGenerator struct {
	rng *rand.Rand
}

func NewMetadataGenerator(rng *rand.Rand) *MetadataGenerator {
	return &MetadataGenerator{rng: rng}
}

func (mg *MetadataGenerator) Generate(nodeType string) string {
	metadata := make(map[string]interface{})

	cpuCores := []int{2, 4, 8, 16, 32, 64, 128}
	metadata["cpu_cores"] = cpuCores[mg.rng.Intn(len(cpuCores))]

	ramGB := []int{4, 8, 16, 32, 64, 128, 256, 512}
	metadata["ram_gb"] = ramGB[mg.rng.Intn(len(ramGB))]

	switch nodeType {
	case "BAREMETAL":
		metadata["hardware_vendor"] = mg.randomFrom([]string{"Dell", "HP", "IBM", "Cisco", "Supermicro"})
		metadata["raid_type"] = mg.randomFrom([]string{"RAID0", "RAID1", "RAID5", "RAID10"})
		metadata["disk_type"] = mg.randomFrom([]string{"SSD", "NVMe", "HDD"})
	case "VM":
		metadata["hypervisor"] = mg.randomFrom([]string{"VMware", "KVM", "Xen", "Hyper-V"})
		metadata["disk_gb"] = mg.randomFrom([]int{100, 250, 500, 1000, 2000})
		metadata["network_type"] = mg.randomFrom([]string{"virtio", "e1000", "vmxnet3"})
	case "CONTAINER":
		metadata["runtime"] = mg.randomFrom([]string{"docker", "containerd", "cri-o", "podman"})
		metadata["image"] = mg.randomFrom([]string{"alpine:latest", "ubuntu:22.04", "nginx:1.25", "redis:7", "postgres:15"})
		metadata["replicas"] = mg.rng.Intn(10) + 1
	}

	metadata["os"] = mg.randomFrom([]string{"Ubuntu 22.04", "RHEL 9", "Debian 12", "Rocky 9", "Alpine 3.18"})
	metadata["kernel_version"] = mg.randomFrom([]string{"5.15.0", "5.19.0", "6.1.0", "6.5.0"})
	metadata["uptime_days"] = mg.rng.Intn(365)
	metadata["load_avg"] = float64(mg.rng.Intn(100)) / 100.0

	owners := []string{"team-alpha", "team-beta", "team-gamma", "team-delta"}
	metadata["owner"] = owners[mg.rng.Intn(len(owners))]

	metadata["monitoring_enabled"] = mg.rng.Float64() < 0.8
	metadata["backup_enabled"] = mg.rng.Float64() < 0.6
	metadata["auto_scaling"] = mg.rng.Float64() < 0.3

	jsonData, _ := json.Marshal(metadata)
	return string(jsonData)
}

func (mg *MetadataGenerator) Update(existing string) string {
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(existing), &metadata); err != nil {
		metadata = make(map[string]interface{})
	}

	metadata["load_avg"] = float64(mg.rng.Intn(100)) / 100.0
	metadata["uptime_days"] = mg.rng.Intn(365)

	if mg.rng.Float64() < 0.3 {
		metadata["last_patched"] = "2024-01-15"
	}

	if mg.rng.Float64() < 0.2 {
		alerts := []string{"none", "disk_space", "high_cpu", "memory_pressure"}
		metadata["active_alerts"] = alerts[mg.rng.Intn(len(alerts))]
	}

	jsonData, _ := json.Marshal(metadata)
	return string(jsonData)
}

func (mg *MetadataGenerator) randomFrom(options interface{}) interface{} {
	switch v := options.(type) {
	case []string:
		return v[mg.rng.Intn(len(v))]
	case []int:
		return v[mg.rng.Intn(len(v))]
	default:
		return nil
	}
}