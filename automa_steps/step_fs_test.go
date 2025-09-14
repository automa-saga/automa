package automa_steps

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/automa-saga/automa"
	"github.com/stretchr/testify/assert"
)

func TestNewMkdirStep(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "testdir")
	step := NewMkdirStep("mkdir", []string{dir}, 0755)
	s, err := step.Build()
	assert.NoError(t, err)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, automa.StatusSuccess, report.Status)
	_, statErr := os.Stat(dir)
	assert.NoError(t, statErr)
}

func TestNewMkdirStep_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	step := NewMkdirStep("mkdir", []string{dir}, 0755)
	s, err := step.Build()
	assert.NoError(t, err)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, automa.StatusSuccess, report.Status)
}

func TestNewRemoveDirStep(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "to_remove")
	assert.NoError(t, os.MkdirAll(dir, 0755))
	step := NewRemoveDirStep("rmdir", []string{dir})
	s, err := step.Build()
	assert.NoError(t, err)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, automa.StatusSuccess, report.Status)
	_, statErr := os.Stat(dir)
	assert.True(t, os.IsNotExist(statErr))
}

func TestNewRemoveDirStep_NotExist(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does_not_exist")
	step := NewRemoveDirStep("rmdir", []string{dir})
	s, err := step.Build()
	assert.NoError(t, err)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, automa.StatusSuccess, report.Status)
}

func TestNewRemoveFileStep(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "file.txt")
	assert.NoError(t, os.WriteFile(file, []byte("data"), 0644))
	step := NewRemoveFileStep("rm_file", file)
	s, err := step.Build()
	assert.NoError(t, err)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, automa.StatusSuccess, report.Status)
	_, statErr := os.Stat(file)
	assert.True(t, os.IsNotExist(statErr))
}

func TestNewRemoveFileStep_NotExist(t *testing.T) {
	file := filepath.Join(t.TempDir(), "no_file.txt")
	step := NewRemoveFileStep("rm_file", file)
	s, err := step.Build()
	assert.NoError(t, err)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, automa.StatusSuccess, report.Status)
}

func TestCopyFile_Success(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.txt")
	dest := filepath.Join(tmp, "dest.txt")
	assert.NoError(t, os.WriteFile(src, []byte("hello"), 0644))
	err := copyFile(src, dest, 0600)
	assert.NoError(t, err)
	data, err := os.ReadFile(dest)
	assert.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestCopyFile_SourceNotExist(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "no.txt")
	dest := filepath.Join(tmp, "dest.txt")
	err := copyFile(src, dest, 0600)
	assert.Error(t, err)
}

func TestCopyFile_DestInvalid(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.txt")
	assert.NoError(t, os.WriteFile(src, []byte("x"), 0644))
	// Use a directory as dest to force error
	err := copyFile(src, tmp, 0600)
	assert.Error(t, err)
}

func TestNewCopyFileStep(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.txt")
	dest := filepath.Join(tmp, "dest.txt")
	assert.NoError(t, os.WriteFile(src, []byte("foo"), 0644))
	step := NewCopyFileStep("copy", src, dest, 0644, true)
	s, err := step.Build()
	assert.NoError(t, err)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, automa.StatusSuccess, report.Status)
	data, err := os.ReadFile(dest)
	assert.NoError(t, err)
	assert.Equal(t, "foo", string(data))
}

func TestNewCopyFileStep_OverwriteFalse(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.txt")
	dest := filepath.Join(tmp, "dest.txt")
	assert.NoError(t, os.WriteFile(src, []byte("foo"), 0644))
	assert.NoError(t, os.WriteFile(dest, []byte("bar"), 0644))
	step := NewCopyFileStep("copy", src, dest, 0644, false)
	s, err := step.Build()
	assert.NoError(t, err)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, automa.StatusSuccess, report.Status)
	// Should not overwrite
	data, err := os.ReadFile(dest)
	assert.NoError(t, err)
	assert.Equal(t, "bar", string(data))
}

func TestNewBackupFileStep(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.txt")
	backupDir := filepath.Join(tmp, "backup")
	assert.NoError(t, os.MkdirAll(backupDir, 0755))
	assert.NoError(t, os.WriteFile(src, []byte("backupme"), 0644))
	step := NewBackupFileStep("backup", src, backupDir, 0644)
	s, err := step.Build()
	assert.NoError(t, err)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, automa.StatusSuccess, report.Status)
	backupFile := filepath.Join(backupDir, "src.txt")
	data, err := os.ReadFile(backupFile)
	assert.NoError(t, err)
	assert.Equal(t, "backupme", string(data))
}

func TestNewBackupFileStep_AlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.txt")
	backupDir := filepath.Join(tmp, "backup")
	backupFile := filepath.Join(backupDir, "src.txt")
	assert.NoError(t, os.MkdirAll(backupDir, 0755))
	assert.NoError(t, os.WriteFile(src, []byte("src"), 0644))
	assert.NoError(t, os.WriteFile(backupFile, []byte("already"), 0644))
	step := NewBackupFileStep("backup", src, backupDir, 0644)
	s, err := step.Build()
	assert.NoError(t, err)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, automa.StatusSuccess, report.Status)
	data, err := os.ReadFile(backupFile)
	assert.NoError(t, err)
	assert.Equal(t, "already", string(data))
}

func TestNewRestoreFileStep(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.txt")
	destDir := filepath.Join(tmp, "dest")
	assert.NoError(t, os.MkdirAll(destDir, 0755))
	assert.NoError(t, os.WriteFile(src, []byte("restoreme"), 0644))
	step := NewRestoreFileStep("restore", src, destDir, 0644)
	s, err := step.Build()
	assert.NoError(t, err)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, automa.StatusSuccess, report.Status)
	destFile := filepath.Join(destDir, "src.txt")
	data, err := os.ReadFile(destFile)
	assert.NoError(t, err)
	assert.Equal(t, "restoreme", string(data))

	// Test rollback removes the file
	report, err = s.OnRollback(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, automa.StatusSuccess, report.Status)
	_, statErr := os.Stat(destFile)
	assert.True(t, os.IsNotExist(statErr))
}
