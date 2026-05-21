package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// PrepareGoModWorkspace ensures go get runs in the directory that contains the
// destination go.mod. When src and dst differ, src (and go.sum when present) is
// copied to dst first; both config paths are then set to the basename of dst.
func PrepareGoModWorkspace() error {
	src, err := filepath.Abs(Config.GoModSrc)
	if err != nil {
		return fmt.Errorf("src-go-mod path: %w", err)
	}
	dst, err := filepath.Abs(Config.GoModDst)
	if err != nil {
		return fmt.Errorf("dst-go-mod path: %w", err)
	}

	if src != dst {
		if err := copyGoModPair(src, dst); err != nil {
			return err
		}
	}

	dir := filepath.Dir(dst)
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("chdir to go.mod directory: %w", err)
	}

	base := filepath.Base(dst)
	Config.GoModSrc = base
	Config.GoModDst = base
	return nil
}

func copyGoModPair(srcMod, dstMod string) error {
	if err := copyFile(srcMod, dstMod); err != nil {
		return fmt.Errorf("copy go.mod: %w", err)
	}
	srcSum := GoSumPath(srcMod)
	dstSum := GoSumPath(dstMod)
	sumData, err := os.ReadFile(srcSum)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.Remove(dstSum); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove go.sum: %w", err)
			}
			return nil
		}
		return fmt.Errorf("read go.sum: %w", err)
	}
	if err := os.WriteFile(dstSum, sumData, 0644); err != nil {
		return fmt.Errorf("write go.sum: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
