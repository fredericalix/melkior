package sim

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type LabelGenerator struct {
	rng         *rand.Rand
	batchID     string
	labelPrefix string
}

func NewLabelGenerator(rng *rand.Rand, labelPrefix string) *LabelGenerator {
	return &LabelGenerator{
		rng:         rng,
		batchID:     fmt.Sprintf("%d", time.Now().Unix()),
		labelPrefix: labelPrefix,
	}
}

func (lg *LabelGenerator) Generate(extraLabels []string) map[string]string {
	labels := make(map[string]string)

	labels["demo"] = "true"
	labels["demo.owner"] = "cli"
	labels["demo.batch"] = lg.batchID
	labels[lg.labelPrefix+"managed"] = "true"

	envs := []string{"dev", "staging", "prod", "test"}
	labels["env"] = envs[lg.rng.Intn(len(envs))]

	dcs := []string{"us-east-1", "us-west-2", "eu-central-1", "ap-southeast-1"}
	labels["datacenter"] = dcs[lg.rng.Intn(len(dcs))]

	services := []string{"api", "web", "db", "cache", "worker", "analytics", "monitoring"}
	labels["service"] = services[lg.rng.Intn(len(services))]

	for _, label := range extraLabels {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
		}
	}

	return labels
}

func (lg *LabelGenerator) UpdateLabels(existing map[string]string) map[string]string {
	updated := make(map[string]string)
	for k, v := range existing {
		updated[k] = v
	}

	operations := []string{"scale", "update", "patch", "rotate", "refresh"}
	updated["last_operation"] = operations[lg.rng.Intn(len(operations))]

	updated["updated_at"] = time.Now().Format(time.RFC3339)

	if lg.rng.Float64() < 0.3 {
		versions := []string{"v1.0", "v1.1", "v2.0", "v2.1", "v3.0"}
		updated["version"] = versions[lg.rng.Intn(len(versions))]
	}

	if lg.rng.Float64() < 0.2 {
		teams := []string{"platform", "infra", "sre", "devops"}
		updated["team"] = teams[lg.rng.Intn(len(teams))]
	}

	return updated
}

func FilterSimulatorLabels(labels map[string]string) bool {
	return labels["demo"] == "true" && labels["demo.owner"] == "cli"
}