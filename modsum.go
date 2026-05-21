package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

// GoSumPath returns go.sum in the same directory as the go.mod file.
func GoSumPath(goModPath string) string {
	return filepath.Join(filepath.Dir(goModPath), "go.sum")
}

// ReadGoSum reads go.sum for the module that owns goModPath.
func ReadGoSum(goModPath string) ([]byte, error) {
	sumPath := GoSumPath(goModPath)
	data, err := os.ReadFile(sumPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("missing %s", sumPath)
		}
		return nil, fmt.Errorf("error reading %s: %w", sumPath, err)
	}
	return data, nil
}

// RestoreModuleState writes mod to goModPath and restores go.sum to sum.
func RestoreModuleState(goModPath string, mod *modfile.File, sum []byte) error {
	if err := SaveMod(goModPath, mod); err != nil {
		return err
	}
	if err := os.WriteFile(GoSumPath(goModPath), sum, 0644); err != nil {
		return fmt.Errorf("error writing go.sum: %w", err)
	}
	return nil
}
