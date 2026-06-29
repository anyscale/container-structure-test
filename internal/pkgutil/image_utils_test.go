package pkgutil

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// testImageWithFile builds a single-layer image whose filesystem contains the
// named file with the given contents.
func testImageWithFile(t *testing.T, name, body string) v1.Image {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(body)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	layer, err := tarball.LayerFromReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// writeOCILayout writes the given images into a fresh OCI layout directory and
// returns its path.
func writeOCILayout(t *testing.T, imgs ...v1.Image) string {
	t.Helper()

	dir := t.TempDir()
	l, err := layout.Write(dir, empty.Index)
	if err != nil {
		t.Fatal(err)
	}
	for _, img := range imgs {
		if err := l.AppendImage(img); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// platformedImage pairs an image with the platform descriptor it should be
// indexed under. A nil platform produces an index entry whose Descriptor.Platform
// is nil, which OCI permits and which must not panic Satisfies.
type platformedImage struct {
	img      v1.Image
	platform *v1.Platform
}

// writeMultiArchOCILayout writes a single multi-arch image index (built from the
// given images and platforms) as the sole entry of a fresh OCI layout directory.
// This mirrors the shape produced by `crane pull --format=oci` of a manifest list.
func writeMultiArchOCILayout(t *testing.T, entries ...platformedImage) string {
	t.Helper()

	idx := v1.ImageIndex(empty.Index)
	for _, e := range entries {
		add := mutate.IndexAddendum{Add: e.img}
		if e.platform != nil {
			add.Descriptor = v1.Descriptor{Platform: e.platform}
		}
		idx = mutate.AppendManifests(idx, add)
	}

	dir := t.TempDir()
	l, err := layout.Write(dir, empty.Index)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AppendIndex(idx); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestImageFromOCILayoutSelectsMatchingPlatform(t *testing.T) {
	amd64 := testImageWithFile(t, "etc/amd64.txt", "amd64-data")
	arm64 := testImageWithFile(t, "etc/arm64.txt", "arm64-data")
	dir := writeMultiArchOCILayout(t,
		platformedImage{img: amd64, platform: &v1.Platform{OS: "linux", Architecture: "amd64"}},
		platformedImage{img: arm64, platform: &v1.Platform{OS: "linux", Architecture: "arm64"}},
	)

	gotImg, _, err := ImageFromOCILayout(dir, "linux/arm64")
	if err != nil {
		t.Fatal(err)
	}

	wantDigest, err := arm64.Digest()
	if err != nil {
		t.Fatal(err)
	}
	gotDigest, err := gotImg.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if gotDigest != wantDigest {
		t.Fatalf("selected image digest = %v, want arm64 image %v", gotDigest, wantDigest)
	}
}

// TestImageFromOCILayoutSkipsNilPlatform guards the nil-deref fix: an index
// descriptor without a platform field must be skipped rather than passed to the
// value-receiver Satisfies, which would panic on a nil *v1.Platform.
func TestImageFromOCILayoutSkipsNilPlatform(t *testing.T) {
	noPlatform := testImageWithFile(t, "etc/nop.txt", "no-platform-data")
	amd64 := testImageWithFile(t, "etc/amd64.txt", "amd64-data")
	dir := writeMultiArchOCILayout(t,
		platformedImage{img: noPlatform, platform: nil},
		platformedImage{img: amd64, platform: &v1.Platform{OS: "linux", Architecture: "amd64"}},
	)

	gotImg, _, err := ImageFromOCILayout(dir, "linux/amd64")
	if err != nil {
		t.Fatal(err)
	}

	wantDigest, err := amd64.Digest()
	if err != nil {
		t.Fatal(err)
	}
	gotDigest, err := gotImg.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if gotDigest != wantDigest {
		t.Fatalf("selected image digest = %v, want amd64 image %v", gotDigest, wantDigest)
	}
}

func TestImageFromV1ExtractsFilesystem(t *testing.T) {
	img := testImageWithFile(t, "etc/known.txt", "image-data")

	result, err := ImageFromV1(img, "test-source")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(result.FSPath)

	if result.Source != "test-source" {
		t.Fatalf("Source = %q, want %q", result.Source, "test-source")
	}
	if result.FSPath == "" {
		t.Fatal("FSPath is empty")
	}
	wantDigest, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if result.Digest != wantDigest {
		t.Fatalf("Digest = %v, want %v", result.Digest, wantDigest)
	}

	got, err := os.ReadFile(filepath.Join(result.FSPath, "etc", "known.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "image-data" {
		t.Fatalf("extracted file = %q, want %q", got, "image-data")
	}
}

func TestImageFromOCILayoutLoadsSingleImage(t *testing.T) {
	img := testImageWithFile(t, "etc/known.txt", "image-data")
	dir := writeOCILayout(t, img)

	gotImg, _, err := ImageFromOCILayout(dir, "linux/amd64")
	if err != nil {
		t.Fatal(err)
	}

	wantDigest, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}
	gotDigest, err := gotImg.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if gotDigest != wantDigest {
		t.Fatalf("loaded image digest = %v, want %v", gotDigest, wantDigest)
	}
}

func TestImageFromOCILayoutMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	_, _, err := ImageFromOCILayout(missing, "linux/amd64")
	if err == nil {
		t.Fatal("ImageFromOCILayout accepted a missing layout path")
	}
}

func TestImageFromOCILayoutRejectsMultipleEntries(t *testing.T) {
	dir := writeOCILayout(t,
		testImageWithFile(t, "etc/a.txt", "a-data"),
		testImageWithFile(t, "etc/b.txt", "b-data"),
	)

	_, _, err := ImageFromOCILayout(dir, "linux/amd64")
	if err == nil {
		t.Fatal("ImageFromOCILayout accepted a layout with multiple entries")
	}
}
