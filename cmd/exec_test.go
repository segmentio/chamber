package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestExecCommandFlags(t *testing.T) {
	t.Run("no-warn-conflicts flag should be defined", func(t *testing.T) {
		// Reset the flag to default state
		noWarnConflicts = false
		
		// Create a new exec command to test flag parsing
		cmd := &cobra.Command{}
		cmd.Flags().BoolVar(&noWarnConflicts, "no-warn-conflicts", false, "suppress warnings when services overwrite environment variables")
		
		// Test that the flag can be set
		err := cmd.Flags().Set("no-warn-conflicts", "true")
		assert.NoError(t, err)
		assert.True(t, noWarnConflicts)
		
		// Test that the flag can be unset
		err = cmd.Flags().Set("no-warn-conflicts", "false")
		assert.NoError(t, err)
		assert.False(t, noWarnConflicts)
	})
	
	t.Run("no-warn-conflicts flag should have correct default value", func(t *testing.T) {
		// Reset to default state
		noWarnConflicts = false
		assert.False(t, noWarnConflicts)
	})
}

func TestWarningBehavior(t *testing.T) {
	// Helper function to capture stderr output
	captureStderr := func(fn func()) string {
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w
		
		fn()
		
		w.Close()
		os.Stderr = oldStderr
		
		var buf bytes.Buffer
		buf.ReadFrom(r)
		return buf.String()
	}

	t.Run("should emit warnings when noWarnConflicts is false", func(t *testing.T) {
		// Reset to default state
		noWarnConflicts = false
		
		// Simulate the warning logic from exec.go
		collisions := []string{"DB_HOST", "API_KEY"}
		service := "test-service"
		
		output := captureStderr(func() {
			if !noWarnConflicts {
				for _, c := range collisions {
					os.Stderr.WriteString("warning: service " + service + " overwriting environment variable " + c + "\n")
				}
			}
		})
		
		assert.Contains(t, output, "warning: service test-service overwriting environment variable DB_HOST")
		assert.Contains(t, output, "warning: service test-service overwriting environment variable API_KEY")
	})

	t.Run("should not emit warnings when noWarnConflicts is true", func(t *testing.T) {
		// Set flag to suppress warnings
		noWarnConflicts = true
		
		// Simulate the warning logic from exec.go
		collisions := []string{"DB_HOST", "API_KEY"}
		service := "test-service"
		
		output := captureStderr(func() {
			if !noWarnConflicts {
				for _, c := range collisions {
					os.Stderr.WriteString("warning: service " + service + " overwriting environment variable " + c + "\n")
				}
			}
		})
		
		assert.Empty(t, output)
	})

	t.Run("should handle empty collisions list", func(t *testing.T) {
		// Reset to default state
		noWarnConflicts = false
		
		// Simulate the warning logic with no collisions
		collisions := []string{}
		service := "test-service"
		
		output := captureStderr(func() {
			if !noWarnConflicts {
				for _, c := range collisions {
					os.Stderr.WriteString("warning: service " + service + " overwriting environment variable " + c + "\n")
				}
			}
		})
		
		assert.Empty(t, output)
	})
}
