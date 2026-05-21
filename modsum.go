package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/mod/modfile"
)

// GoSumPath returns the go.sum path adjacent to a go.mod path.
func GoSumPath(goModPath string) string {
	return strings.TrimSuffix(goModPath, ".mod") + ".sum"
}

// ReadGoSumSnapshot reads go.sum bytes beside goModPath. hadSum is false when the file is absent.
func ReadGoSumSnapshot(goModPath string) (data []byte, hadSum bool, err error) {
	sumPath := GoSumPath(goModPath)
	data, err = os.ReadFile(sumPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("error reading go.sum: %w", err)
	}
	return data, true, nil
}

// RestoreModuleState writes mod to goModPath and restores go.sum to the given snapshot.
func RestoreModuleState(goModPath string, mod *modfile.File, sum []byte, hadSum bool) error {
	if err := SaveMod(goModPath, mod); err != nil {
		return err
	}
	sumPath := GoSumPath(goModPath)
	if !hadSum {
		if err := os.Remove(sumPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("error removing go.sum: %w", err)
		}
		return nil
	}
	if err := os.WriteFile(sumPath, sum, 0644); err != nil {
		return fmt.Errorf("error writing go.sum: %w", err)
	}
	return nil
}
