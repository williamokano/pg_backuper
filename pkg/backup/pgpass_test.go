package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPgpassPath(t *testing.T) {
	t.Run("explicit_path_exists", func(t *testing.T) {
		// Create temp .pgpass file
		tmpFile, err := os.CreateTemp("", ".pgpass_*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Test with explicit path
		path, err := GetPgpassPath(tmpFile.Name())
		require.NoError(t, err)
		assert.Equal(t, tmpFile.Name(), path)
	})

	t.Run("explicit_path_not_found", func(t *testing.T) {
		// Test with non-existent path
		path, err := GetPgpassPath("/nonexistent/.pgpass")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configured pgpass file not found")
		assert.Empty(t, path)
	})

	t.Run("no_explicit_path", func(t *testing.T) {
		// This test depends on environment - just ensure it doesn't crash
		path, err := GetPgpassPath("")

		// May or may not find a file depending on environment
		if err != nil {
			assert.Contains(t, err.Error(), "no .pgpass file found")
		} else {
			assert.NotEmpty(t, path)
		}
	})
}

func TestValidatePgpassPermissions(t *testing.T) {
	t.Run("correct_permissions_0600", func(t *testing.T) {
		// Create temp .pgpass file with 0600
		tmpFile, err := os.CreateTemp("", ".pgpass_*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Set correct permissions
		err = os.Chmod(tmpFile.Name(), 0600)
		require.NoError(t, err)

		// Validate
		err = ValidatePgpassPermissions(tmpFile.Name())
		assert.NoError(t, err)
	})

	t.Run("wrong_permissions_0644", func(t *testing.T) {
		// Create temp .pgpass file with 0644
		tmpFile, err := os.CreateTemp("", ".pgpass_*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Set wrong permissions
		err = os.Chmod(tmpFile.Name(), 0644)
		require.NoError(t, err)

		// Validate
		err = ValidatePgpassPermissions(tmpFile.Name())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "incorrect permissions")
		assert.Contains(t, err.Error(), "must be 0600")
	})

	t.Run("wrong_permissions_0777", func(t *testing.T) {
		// Create temp .pgpass file with 0777
		tmpFile, err := os.CreateTemp("", ".pgpass_*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Set wrong permissions
		err = os.Chmod(tmpFile.Name(), 0777)
		require.NoError(t, err)

		// Validate
		err = ValidatePgpassPermissions(tmpFile.Name())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "incorrect permissions")
	})

	t.Run("file_not_found", func(t *testing.T) {
		err := ValidatePgpassPermissions("/nonexistent/.pgpass")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to stat")
	})
}

func TestVerifyPgpassEntry(t *testing.T) {
	t.Run("entry_found_exact_match", func(t *testing.T) {
		// Create temp .pgpass with specific entry
		tmpFile, err := os.CreateTemp("", ".pgpass_*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		content := "localhost:5432:testdb:postgres:testpass\n"
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
		tmpFile.Close()

		// Verify entry exists
		found, err := VerifyPgpassEntry(tmpFile.Name(), "localhost", "5432", "testdb", "postgres")
		require.NoError(t, err)
		assert.True(t, found)
	})

	t.Run("entry_found_with_wildcards", func(t *testing.T) {
		// Create temp .pgpass with wildcard entry
		tmpFile, err := os.CreateTemp("", ".pgpass_*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		content := "*:*:testdb:postgres:testpass\n"
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
		tmpFile.Close()

		// Verify entry matches with wildcards
		found, err := VerifyPgpassEntry(tmpFile.Name(), "localhost", "5432", "testdb", "postgres")
		require.NoError(t, err)
		assert.True(t, found)
	})

	t.Run("entry_not_found", func(t *testing.T) {
		// Create temp .pgpass with different entry
		tmpFile, err := os.CreateTemp("", ".pgpass_*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		content := "localhost:5432:otherdb:postgres:testpass\n"
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
		tmpFile.Close()

		// Verify entry not found
		found, err := VerifyPgpassEntry(tmpFile.Name(), "localhost", "5432", "testdb", "postgres")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("empty_file", func(t *testing.T) {
		// Create empty .pgpass
		tmpFile, err := os.CreateTemp("", ".pgpass_*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Verify entry not found
		found, err := VerifyPgpassEntry(tmpFile.Name(), "localhost", "5432", "testdb", "postgres")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("file_with_comments", func(t *testing.T) {
		// Create .pgpass with comments
		tmpFile, err := os.CreateTemp("", ".pgpass_*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		content := `# Comment line
# Another comment
localhost:5432:testdb:postgres:testpass
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
		tmpFile.Close()

		// Verify entry found (comments ignored)
		found, err := VerifyPgpassEntry(tmpFile.Name(), "localhost", "5432", "testdb", "postgres")
		require.NoError(t, err)
		assert.True(t, found)
	})

	t.Run("file_with_empty_lines", func(t *testing.T) {
		// Create .pgpass with empty lines
		tmpFile, err := os.CreateTemp("", ".pgpass_*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		content := `
localhost:5432:testdb:postgres:testpass

`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
		tmpFile.Close()

		// Verify entry found (empty lines ignored)
		found, err := VerifyPgpassEntry(tmpFile.Name(), "localhost", "5432", "testdb", "postgres")
		require.NoError(t, err)
		assert.True(t, found)
	})

	t.Run("file_with_invalid_lines", func(t *testing.T) {
		// Create .pgpass with invalid format
		tmpFile, err := os.CreateTemp("", ".pgpass_*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		content := `invalid:line:format
localhost:5432:testdb:postgres:testpass
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
		tmpFile.Close()

		// Verify entry found (invalid lines skipped)
		found, err := VerifyPgpassEntry(tmpFile.Name(), "localhost", "5432", "testdb", "postgres")
		require.NoError(t, err)
		assert.True(t, found)
	})

	t.Run("file_not_found", func(t *testing.T) {
		found, err := VerifyPgpassEntry("/nonexistent/.pgpass", "localhost", "5432", "testdb", "postgres")
		require.Error(t, err)
		assert.False(t, found)
		assert.Contains(t, err.Error(), "failed to open")
	})

	t.Run("multiple_entries", func(t *testing.T) {
		// Create .pgpass with multiple entries
		tmpFile, err := os.CreateTemp("", ".pgpass_*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		content := `localhost:5432:db1:user1:pass1
localhost:5432:testdb:postgres:testpass
remotehost:5432:db2:user2:pass2
`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
		tmpFile.Close()

		// Verify correct entry found
		found, err := VerifyPgpassEntry(tmpFile.Name(), "localhost", "5432", "testdb", "postgres")
		require.NoError(t, err)
		assert.True(t, found)
	})
}

func TestMatchField(t *testing.T) {
	t.Run("exact_match", func(t *testing.T) {
		assert.True(t, matchField("localhost", "localhost"))
		assert.True(t, matchField("5432", "5432"))
	})

	t.Run("wildcard_match", func(t *testing.T) {
		assert.True(t, matchField("*", "localhost"))
		assert.True(t, matchField("*", "any-value"))
		assert.True(t, matchField("*", ""))
	})

	t.Run("no_match", func(t *testing.T) {
		assert.False(t, matchField("localhost", "remotehost"))
		assert.False(t, matchField("5432", "3306"))
	})
}

func TestPgpassWorkflow(t *testing.T) {
	t.Run("full_workflow", func(t *testing.T) {
		// Create temporary directory
		tmpDir, err := os.MkdirTemp("", "pgpass_test_*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create .pgpass file
		pgpassPath := filepath.Join(tmpDir, ".pgpass")
		content := "localhost:5432:testdb:postgres:secretpass\n"
		err = os.WriteFile(pgpassPath, []byte(content), 0600)
		require.NoError(t, err)

		// Test GetPgpassPath with explicit path
		foundPath, err := GetPgpassPath(pgpassPath)
		require.NoError(t, err)
		assert.Equal(t, pgpassPath, foundPath)

		// Test ValidatePgpassPermissions
		err = ValidatePgpassPermissions(foundPath)
		assert.NoError(t, err)

		// Test VerifyPgpassEntry
		found, err := VerifyPgpassEntry(foundPath, "localhost", "5432", "testdb", "postgres")
		require.NoError(t, err)
		assert.True(t, found)

		// Test with wrong database
		found, err = VerifyPgpassEntry(foundPath, "localhost", "5432", "wrongdb", "postgres")
		require.NoError(t, err)
		assert.False(t, found)
	})
}
