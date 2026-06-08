package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

func goSumPath() string {
	return filepath.Join(filepath.Dir(goModFile), "go.sum")
}

// ReadGoSum reads go.sum for the module in the current directory.
func ReadGoSum() ([]byte, error) {
	sumPath := goSumPath()
	data, err := os.ReadFile(sumPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("missing %s", sumPath)
		}
		return nil, fmt.Errorf("error reading %s: %w", sumPath, err)
	}
	return data, nil
}

// RestoreModuleState writes mod and restores go.sum.
func RestoreModuleState(mod *modfile.File, sum []byte) error {
	if err := SaveMod(goModFile, mod); err != nil {
		return err
	}
	if err := os.WriteFile(goSumPath(), sum, 0644); err != nil {
		return fmt.Errorf("error writing go.sum: %w", err)
	}
	return nil
}
