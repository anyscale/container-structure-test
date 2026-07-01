package pkgutil

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func testTarReader(t *testing.T, entries ...func(*tar.Writer) error) *tar.Reader {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, entry := range entries {
		if err := entry(tw); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return tar.NewReader(bytes.NewReader(buf.Bytes()))
}

func testTarFile(name, body string) func(*tar.Writer) error {
	return func(tw *tar.Writer) error {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(body)),
		}); err != nil {
			return err
		}
		_, err := tw.Write([]byte(body))
		return err
	}
}

func testTarDir(name string) func(*tar.Writer) error {
	return func(tw *tar.Writer) error {
		return tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0755,
			Typeflag: tar.TypeDir,
		})
	}
}

func testTarDirMode(name string, mode int64) func(*tar.Writer) error {
	return func(tw *tar.Writer) error {
		return tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     mode,
			Typeflag: tar.TypeDir,
		})
	}
}

func testTarSymlink(name, linkname string) func(*tar.Writer) error {
	return func(tw *tar.Writer) error {
		return tw.WriteHeader(&tar.Header{
			Name:     name,
			Linkname: linkname,
			Typeflag: tar.TypeSymlink,
		})
	}
}

func testTarHardlink(name, linkname string) func(*tar.Writer) error {
	return func(tw *tar.Writer) error {
		return tw.WriteHeader(&tar.Header{
			Name:     name,
			Linkname: linkname,
			Typeflag: tar.TypeLink,
		})
	}
}

func TestUnpackTarRejectsEntriesOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}

	err := unpackTar(testTarReader(t, testTarFile("../outside.txt", "outside-data")), root, nil)
	if err == nil {
		t.Fatal("unpackTar allowed an entry outside the root")
	}
	if _, err := os.Stat(filepath.Join(parent, "outside.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside path exists after rejected unpack: %v", err)
	}
}

func TestUnpackTarRejectsSymlinksOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}

	err := unpackTar(testTarReader(t, testTarSymlink("link", "..")), root, nil)
	if err == nil {
		t.Fatal("unpackTar allowed a symlink outside the root")
	}
}

func TestUnpackTarRejectsHardlinksOutsideRoot(t *testing.T) {
	root := t.TempDir()

	err := unpackTar(testTarReader(t, testTarHardlink("link", "../outside.txt")), root, nil)
	if err == nil {
		t.Fatal("unpackTar allowed a hard link outside the root")
	}
}

func TestUnpackTarHardlinkToSymlinkDoesNotFollowTarget(t *testing.T) {
	root := t.TempDir()

	err := unpackTar(testTarReader(t,
		testTarFile("file.txt", "image-data"),
		testTarSymlink("sym", "file.txt"),
		testTarHardlink("hard", "sym"),
	), root, nil)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(filepath.Join(root, "hard"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("hard link to symlink should remain a symlink, got mode %s", info.Mode())
	}
	target, err := os.Readlink(filepath.Join(root, "hard"))
	if err != nil {
		t.Fatal(err)
	}
	if target != "file.txt" {
		t.Fatalf("hard link to symlink target = %q", target)
	}
}

func TestUnpackTarRejectsHardlinkSourceSymlinkOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "outside.txt"), []byte("outside-data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside.txt", filepath.Join(root, "sym")); err != nil {
		t.Fatal(err)
	}

	err := unpackTar(testTarReader(t, testTarHardlink("hard", "sym")), root, nil)
	if err == nil {
		t.Fatal("unpackTar allowed a hard link through a symlink outside the root")
	}
	if _, err := os.Lstat(filepath.Join(root, "hard")); !os.IsNotExist(err) {
		t.Fatalf("hard link path exists after rejected unpack: %v", err)
	}
}

func TestUnpackTarReadsFileUnderNonTraversableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory traversal permissions; bug only manifests for the non-root test user")
	}
	root := t.TempDir()

	// A directory shipped without the owner execute bit (e.g. /licenses at 0644)
	// can neither be populated during extraction nor traversed to inspect the
	// files inside it when running as the non-root test user.
	err := unpackTar(testTarReader(t,
		testTarDirMode("licenses", 0644),
		testTarFile("licenses/copyright", "license-text"),
	), root, nil)
	if err != nil {
		t.Fatalf("unpackTar failed on a file under a non-traversable dir: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, "licenses", "copyright"))
	if err != nil {
		t.Fatalf("reading file under non-traversable dir: %v", err)
	}
	if string(got) != "license-text" {
		t.Fatalf("file content = %q, want %q", got, "license-text")
	}
}

func TestUnpackTarForcesOwnerExecuteOnNonTraversableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory traversal permissions; bug only manifests for the non-root test user")
	}
	root := t.TempDir()

	if err := unpackTar(testTarReader(t,
		testTarDirMode("licenses", 0644),
		testTarFile("licenses/copyright", "license-text"),
	), root, nil); err != nil {
		t.Fatal(err)
	}

	// Owner-execute is forced on so cst can traverse the dir to inspect its
	// contents; the remaining shipped bits are preserved (0644 -> 0744).
	info, err := os.Lstat(filepath.Join(root, "licenses"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0744 {
		t.Fatalf("non-traversable dir reported mode = %o, want 0744", got)
	}
}

func TestUnpackTarPreservesModeOfTraversableDir(t *testing.T) {
	root := t.TempDir()
	// Runs before TempDir's own RemoveAll (cleanups are LIFO): make the write-less
	// dir writable again so the non-root test user can unlink its contents.
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "app"), 0755) })

	// A write-less but already-traversable directory (0555) is the common case
	// the temp-permission logic exists for; its mode must round-trip exactly.
	if err := unpackTar(testTarReader(t,
		testTarDirMode("app", 0555),
		testTarFile("app/run", "bin"),
	), root, nil); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(filepath.Join(root, "app"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0555 {
		t.Fatalf("traversable dir reported mode = %o, want 0555", got)
	}
}

func TestUnpackTarExtractsSetuidFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can set setuid unconditionally; the bug only manifests for the non-root test user")
	}
	root := t.TempDir()

	// A setuid regular file (e.g. /usr/bin/chfn) must extract successfully. On
	// platforms where a non-root user cannot chmod the setuid bit (macOS/BSD),
	// the initial chmod fails and the graceful fallback re-chmods without the
	// special bits instead of aborting. On Linux the first chmod succeeds and the
	// bit is preserved. Either way, extraction must not error and the file must be
	// readable so tests inside such an image can run.
	err := unpackTar(testTarReader(t, func(tw *tar.Writer) error {
		body := "bin"
		if err := tw.WriteHeader(&tar.Header{
			Name:     "usr/bin/chfn",
			Mode:     04755, // setuid
			Size:     int64(len(body)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			return err
		}
		_, err := tw.Write([]byte(body))
		return err
	}), root, nil)
	if err != nil {
		t.Fatalf("unpackTar failed on a setuid file: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, "usr", "bin", "chfn"))
	if err != nil {
		t.Fatalf("reading extracted setuid file: %v", err)
	}
	if string(got) != "bin" {
		t.Fatalf("setuid file content = %q, want %q", got, "bin")
	}
}

func TestUnpackTarResolvesSymlinksWithinRoot(t *testing.T) {
	root := t.TempDir()

	err := unpackTar(testTarReader(t,
		testTarDir("etc"),
		testTarSymlink("link", "/etc"),
		testTarFile("link/generated.txt", "image-data"),
	), root, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(root, "etc", "generated.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "image-data" {
		t.Fatalf("unpacked file through image symlink = %q", got)
	}
}
