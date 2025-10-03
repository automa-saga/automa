package automa_steps

import (
	"context"
	"fmt"
	"github.com/automa-saga/automa"
	"io"
	"io/fs"
	"os"
	"path"
)

func NewMkdirStep(id string, dirs []string, perm fs.FileMode, opts ...automa.StepOption) automa.Builder {
	newOpts := append([]automa.StepOption{}, opts...)
	newOpts = append(newOpts, automa.WithOnExecute(func(ctx context.Context) (*automa.Report, error) {
		for _, dir := range dirs {
			_, err := os.Stat(dir)
			if err == nil {
				return automa.SuccessReport(id), nil
			}

			err = os.MkdirAll(dir, perm)
			if err != nil {
				return nil, automa.StepExecutionError.New("failed to create directory %s: %v", dir, err)
			}
		}

		return automa.SuccessReport(id), nil
	}))
	return automa.NewStepBuilder(id, newOpts...)
}

func NewRemoveDirStep(id string, dirs []string, opts ...automa.StepOption) automa.Builder {
	newOpts := append([]automa.StepOption{}, opts...)
	newOpts = append(newOpts, automa.WithOnExecute(func(ctx context.Context) (*automa.Report, error) {
		for _, dir := range dirs {
			_, err := os.Stat(dir)
			if os.IsNotExist(err) {
				// directory does not exist, nothing to do
				return automa.SuccessReport(id), nil
			}

			err = os.RemoveAll(dir)
			if err != nil {
				return nil, automa.StepExecutionError.New("failed to remove directory %s: %v", dir, err)
			}
		}

		return automa.SuccessReport(id), nil
	}))
	return automa.NewStepBuilder(id, newOpts...)
}

func NewRemoveFileStep(id, filePath string, opts ...automa.StepOption) automa.Builder {
	newOpts := append([]automa.StepOption{}, opts...)
	newOpts = append(newOpts, automa.WithOnExecute(func(ctx context.Context) (*automa.Report, error) {
		_, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			// file does not exist, nothing to do
			return automa.SuccessReport(id), nil
		}

		err = os.Remove(filePath)
		if err != nil {
			return nil, automa.StepExecutionError.New("failed to remove file %s: %v", filePath, err)
		}

		return automa.SuccessReport(id), nil
	}))
	return automa.NewStepBuilder(id, newOpts...)
}

func copyFile(src, dest string, perm fs.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return automa.StepExecutionError.New("failed to open source file %s: %v", src, err)
	}
	defer func(srcFile *os.File) {
		err2 := srcFile.Close()
		if err2 != nil {
			fmt.Printf("WARN: failed to close source file %s: %v", src, err2)
		}
	}(srcFile)

	destFile, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return automa.StepExecutionError.New("failed to create destination file %s: %v", dest, err)
	}
	defer func(destFile *os.File) {
		err2 := destFile.Close()
		if err2 != nil {
			fmt.Printf("WARN: failed to close destination file %s: %v", dest, err2)
		}
	}(destFile)

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return automa.StepExecutionError.New("failed to copy from %s to %s: %v", src, dest, err)
	}

	return nil
}

// NewCopyFileStep creates a step that copies a file from src to dest.
// If the destination file already exists, it will be overwritten if overwrite is true.
// It does not set up any rollback behavior.
func NewCopyFileStep(id, src, dest string, perm fs.FileMode, overwrite bool, opts ...automa.StepOption) automa.Builder {
	newOpts := append([]automa.StepOption{}, opts...)
	newOpts = append(newOpts, automa.WithOnExecute(func(ctx context.Context) (*automa.Report, error) {
		info, err := os.Stat(src)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, automa.StepExecutionError.New("failed to stat source file %s: %v", src, err)
			}
		}

		if info == nil || overwrite {
			err = copyFile(src, dest, perm)
			if err != nil {
				return nil, err
			}
		}

		return automa.SuccessReport(id), nil
	}))

	return automa.NewStepBuilder(id, newOpts...)
}

// NewBackupFileStep creates a step that copies a file from src to a backup directory.
// The backup file will have the same base name as the source file.
// If the backup file already exists, it will not overwrite the file.
// It does not set up any rollback behavior.
func NewBackupFileStep(id, src, backupDir string, perm fs.FileMode, opts ...automa.StepOption) automa.Builder {
	backupPath := path.Join(backupDir, path.Base(src))

	newOpts := append([]automa.StepOption{}, opts...)
	newOpts = append(newOpts, automa.WithOnExecute(func(ctx context.Context) (*automa.Report, error) {
		_, err := os.Stat(backupPath)
		if err == nil {
			// backup file already exists, do not overwrite
			return automa.SuccessReport(id), nil
		}

		err = copyFile(src, backupPath, perm)
		if err != nil {
			return nil, err
		}

		return automa.SuccessReport(id), nil
	}))

	return automa.NewStepBuilder(id, newOpts...)
}

// NewRestoreFileStep creates a step that copies a file from src to dest.
// On rollback, it removes the restored file from dest if it exists.
// If the destination file already exists, it will be overwritten.
func NewRestoreFileStep(id, src, destDir string, perm fs.FileMode, opts ...automa.StepOption) automa.Builder {
	dest := path.Join(destDir, path.Base(src))

	newOpts := append([]automa.StepOption{}, opts...)
	newOpts = append(newOpts, automa.WithOnRollback(func(ctx context.Context) (*automa.Report, error) {
		// on rollback, remove the restored file
		_, err := os.Stat(dest)
		if os.IsNotExist(err) {
			// restored file does not exist, nothing to do
			return automa.SuccessReport(id), nil
		}

		err = os.Remove(dest)
		if err != nil {
			return nil, automa.StepExecutionError.New("failed to remove backup file %s: %v", dest, err)
		}

		return automa.SuccessReport(id), nil
	}))

	return NewCopyFileStep(id, src, dest, perm, true, newOpts...)
}
