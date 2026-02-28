package import_manager

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseRemapFlag(flag string) (map[int]int, error) {
	return parseRemapFlag(flag)
}

func parseRemapFlag(flag string) (map[int]int, error) {
	remap := make(map[int]int)
	trimmed := strings.TrimSpace(flag)
	if trimmed == "" {
		return remap, nil
	}

	pairs := strings.Split(trimmed, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			return nil, fmt.Errorf("invalid remap format")
		}

		parts := strings.Split(pair, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid remap pair %q", pair)
		}

		src, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid source display number %q", parts[0])
		}
		dst, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid target display number %q", parts[1])
		}
		if src < 0 || dst < 0 {
			return nil, fmt.Errorf("display number must be non-negative")
		}

		remap[src] = dst
	}

	return remap, nil
}

func applyRemap(displayNum int, remap map[int]int) int {
	if remap == nil {
		return displayNum
	}
	if mapped, ok := remap[displayNum]; ok {
		return mapped
	}
	return displayNum
}
