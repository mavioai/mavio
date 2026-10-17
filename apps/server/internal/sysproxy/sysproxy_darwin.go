package sysproxy

import (
	"context"
	"fmt"
	"os/exec"
)

func fromOS(ctx context.Context) (Setting, error) {
	out, err := exec.CommandContext(ctx, "scutil", "--proxy").Output()
	if err != nil {
		return Setting{}, fmt.Errorf("scutil --proxy: %w", err)
	}
	return ParseScutil(out)
}
