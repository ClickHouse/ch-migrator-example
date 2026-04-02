package migrations

import (
	"io"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// =============================================================================
// templatedFS / templatedFile tests
// =============================================================================

func TestTemplatedFile_Read_BasicReplacement(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		placeholder := rapid.SampledFrom([]string{
			"<SMT_ENGINE>",
			"<ReplacingMergeTree_ENGINE>",
			"<SummingMergeTree_ENGINE>",
			"<AggregatingMergeTree_ENGINE>",
			"<CollapsingMergeTree_ENGINE>",
		}).Draw(t, "placeholder")
		replacement := rapid.SampledFrom([]string{
			"MergeTree()",
			"SharedMergeTree()",
			"ReplacingMergeTree",
			"SharedReplacingMergeTree",
			"SummingMergeTree",
			"SharedSummingMergeTree",
			"AggregatingMergeTree",
			"SharedAggregatingMergeTree",
			"CollapsingMergeTree",
			"SharedCollapsingMergeTree",
		}).Draw(t, "replacement")
		prefix := rapid.StringMatching(`[a-zA-Z ]{0,50}`).Draw(t, "prefix")
		suffix := rapid.StringMatching(`[a-zA-Z ]{0,50}`).Draw(t, "suffix")

		content := prefix + placeholder + suffix
		expected := prefix + replacement + suffix

		mapFS := fstest.MapFS{"test.sql": &fstest.MapFile{Data: []byte(content)}}
		tfs := templatedFS{fs: mapFS, replacements: map[string]string{placeholder: replacement}}

		f, err := tfs.Open("test.sql")
		require.NoError(t, err)
		defer f.Close()

		got, err := io.ReadAll(f)
		require.NoError(t, err)
		require.Equal(t, expected, string(got))
	})
}

func TestTemplatedFile_Read_MultipleReplacements(t *testing.T) {
	content := "CREATE TABLE t ENGINE = <SMT_ENGINE> AS SELECT <SMT_ENGINE>"
	replacements := map[string]string{
		"<SMT_ENGINE>": "MergeTree()",
	}
	expected := "CREATE TABLE t ENGINE = MergeTree() AS SELECT MergeTree()"

	mapFS := fstest.MapFS{"test.sql": &fstest.MapFile{Data: []byte(content)}}
	tfs := templatedFS{fs: mapFS, replacements: replacements}

	f, err := tfs.Open("test.sql")
	require.NoError(t, err)
	defer f.Close()

	got, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, expected, string(got))
}

func TestTemplatedFile_Read_RespectsIOReaderContract(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Short placeholder replaced with long string — the old bug
		shortPlaceholder := "<X>"
		longReplacement := rapid.StringMatching(`[A-Za-z()'/,{} ]{20,100}`).Draw(t, "long")
		repeatCount := rapid.IntRange(1, 5).Draw(t, "repeat")
		content := strings.Repeat(shortPlaceholder, repeatCount)

		mapFS := fstest.MapFS{"t.sql": &fstest.MapFile{Data: []byte(content)}}
		tfs := templatedFS{fs: mapFS, replacements: map[string]string{shortPlaceholder: longReplacement}}

		f, err := tfs.Open("t.sql")
		require.NoError(t, err)
		defer f.Close()

		// Read with a small buffer — should never return n > len(buf)
		buf := make([]byte, 8)
		var total []byte
		for {
			n, err := f.Read(buf)
			require.LessOrEqual(t, n, len(buf), "Read must not return n > len(buf)")
			total = append(total, buf[:n]...)
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		expected := strings.ReplaceAll(content, shortPlaceholder, longReplacement)
		require.Equal(t, expected, string(total))
	})
}

func TestTemplatedFile_Read_DoesNotReplaceStaleBufferContent(t *testing.T) {
	// File content does NOT contain a placeholder
	mapFS := fstest.MapFS{"t.sql": &fstest.MapFile{Data: []byte("SELECT 1")}}
	tfs := templatedFS{fs: mapFS, replacements: map[string]string{"<ENGINE>": "MergeTree()"}}

	f, err := tfs.Open("t.sql")
	require.NoError(t, err)
	defer f.Close()

	// Pre-fill buffer with placeholder content (simulating buffer reuse)
	buf := make([]byte, 256)
	copy(buf, "<ENGINE>leftover<ENGINE>")

	n, err := f.Read(buf)
	require.True(t, err == nil || err == io.EOF)

	result := string(buf[:n])
	require.Equal(t, "SELECT 1", result)
	require.NotContains(t, result, "MergeTree()")
}

func TestTemplatedFS_ReadFile_AppliesReplacements(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		old := "<PLACEHOLDER>"
		new_ := rapid.StringMatching(`[A-Za-z]{5,30}`).Draw(t, "replacement")
		content := "ENGINE = <PLACEHOLDER>"

		mapFS := fstest.MapFS{"m.sql": &fstest.MapFile{Data: []byte(content)}}
		tfs := templatedFS{fs: mapFS, replacements: map[string]string{old: new_}}

		// ReadFile must apply replacements (matches Open+ReadAll behavior)
		rfData, err := tfs.ReadFile("m.sql")
		require.NoError(t, err)

		f, err := tfs.Open("m.sql")
		require.NoError(t, err)
		oData, err := io.ReadAll(f)
		f.Close()
		require.NoError(t, err)

		require.Equal(t, string(oData), string(rfData), "ReadFile and Open+ReadAll must produce identical results")
	})
}

func TestTemplatedFile_Read_EmptyFile(t *testing.T) {
	mapFS := fstest.MapFS{"empty.sql": &fstest.MapFile{Data: []byte("")}}
	tfs := templatedFS{fs: mapFS, replacements: map[string]string{"<X>": "Y"}}

	f, _ := tfs.Open("empty.sql")
	defer f.Close()

	buf := make([]byte, 10)
	n, err := f.Read(buf)
	require.Equal(t, 0, n)
	require.Equal(t, io.EOF, err)
}

func TestTemplatedFile_Read_NoReplacementsNeeded(t *testing.T) {
	content := "SELECT 1 FROM system.tables"
	mapFS := fstest.MapFS{"t.sql": &fstest.MapFile{Data: []byte(content)}}
	tfs := templatedFS{fs: mapFS, replacements: map[string]string{"<NOMATCH>": "replaced"}}

	f, _ := tfs.Open("t.sql")
	defer f.Close()

	got, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, content, string(got))
}

func TestTemplatedFile_Read_ReplacementGrowsContent(t *testing.T) {
	// Specifically tests that large replacements work correctly
	content := "<E>"
	replacement := strings.Repeat("A", 10000)

	mapFS := fstest.MapFS{"t.sql": &fstest.MapFile{Data: []byte(content)}}
	tfs := templatedFS{fs: mapFS, replacements: map[string]string{"<E>": replacement}}

	f, _ := tfs.Open("t.sql")
	defer f.Close()

	got, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, replacement, string(got))
}

func TestTemplatedFile_Read_ReplacementShrinksContent(t *testing.T) {
	content := strings.Repeat("LONGPLACEHOLDER", 100)
	mapFS := fstest.MapFS{"t.sql": &fstest.MapFile{Data: []byte(content)}}
	tfs := templatedFS{fs: mapFS, replacements: map[string]string{"LONGPLACEHOLDER": "X"}}

	f, _ := tfs.Open("t.sql")
	defer f.Close()

	got, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("X", 100), string(got))
}
