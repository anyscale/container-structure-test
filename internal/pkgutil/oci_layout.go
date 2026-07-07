/*
Copyright 2018 Google, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pkgutil

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"
)

// openOCILayout opens the OCI image layout at path, reading blobs with
// os.Open so blob files may be symlinks. go-containerregistry's layout.Path
// refuses to open symlinked blobs, but Bazel's rules_oci assembles oci_load
// layouts whose blobs are symlinks into the action cache; following them
// keeps the layout readable in place, with no staging or copying.
func openOCILayout(path string) (v1.ImageIndex, error) {
	raw, err := os.ReadFile(filepath.Join(path, "index.json"))
	if err != nil {
		return nil, errors.Wrapf(err, "reading index.json of OCI layout %s", path)
	}
	return newLayoutIndex(ociLayout{root: path}, raw)
}

// layoutIndex is a v1.ImageIndex whose manifests and blobs resolve against an
// ociLayout.
type layoutIndex struct {
	layout ociLayout
	raw    []byte
	index  *v1.IndexManifest
}

var _ v1.ImageIndex = (*layoutIndex)(nil)

func newLayoutIndex(l ociLayout, raw []byte) (*layoutIndex, error) {
	index, err := v1.ParseIndexManifest(bytes.NewReader(raw))
	if err != nil {
		return nil, errors.Wrap(err, "parsing index manifest")
	}
	return &layoutIndex{layout: l, raw: raw, index: index}, nil
}

func (i *layoutIndex) MediaType() (types.MediaType, error) {
	if i.index.MediaType != "" {
		return i.index.MediaType, nil
	}
	return types.OCIImageIndex, nil
}

func (i *layoutIndex) Digest() (v1.Hash, error) {
	h, _, err := v1.SHA256(bytes.NewReader(i.raw))
	return h, err
}

func (i *layoutIndex) Size() (int64, error) {
	return int64(len(i.raw)), nil
}

func (i *layoutIndex) IndexManifest() (*v1.IndexManifest, error) {
	return i.index, nil
}

func (i *layoutIndex) RawManifest() ([]byte, error) {
	return i.raw, nil
}

func (i *layoutIndex) Image(h v1.Hash) (v1.Image, error) {
	raw, err := i.layout.bytes(h)
	if err != nil {
		return nil, errors.Wrapf(err, "reading manifest %s", h)
	}
	img, err := newLayoutImage(i.layout, raw)
	if err != nil {
		return nil, errors.Wrapf(err, "loading image %s", h)
	}
	return partial.CompressedToImage(img)
}

func (i *layoutIndex) ImageIndex(h v1.Hash) (v1.ImageIndex, error) {
	raw, err := i.layout.bytes(h)
	if err != nil {
		return nil, errors.Wrapf(err, "reading index manifest %s", h)
	}
	return newLayoutIndex(i.layout, raw)
}

// layoutImage is a partial.CompressedImageCore whose config and layer blobs
// resolve against an ociLayout.
type layoutImage struct {
	layout   ociLayout
	raw      []byte
	manifest *v1.Manifest
}

var _ partial.CompressedImageCore = (*layoutImage)(nil)

func newLayoutImage(l ociLayout, raw []byte) (*layoutImage, error) {
	manifest, err := v1.ParseManifest(bytes.NewReader(raw))
	if err != nil {
		return nil, errors.Wrap(err, "parsing image manifest")
	}
	return &layoutImage{layout: l, raw: raw, manifest: manifest}, nil
}

func (im *layoutImage) MediaType() (types.MediaType, error) {
	if im.manifest.MediaType != "" {
		return im.manifest.MediaType, nil
	}
	return types.OCIManifestSchema1, nil
}

func (im *layoutImage) RawManifest() ([]byte, error) {
	return im.raw, nil
}

func (im *layoutImage) RawConfigFile() ([]byte, error) {
	return im.layout.bytes(im.manifest.Config.Digest)
}

func (im *layoutImage) LayerByDigest(h v1.Hash) (partial.CompressedLayer, error) {
	if h == im.manifest.Config.Digest {
		return layoutLayer{layout: im.layout, desc: im.manifest.Config}, nil
	}
	for _, desc := range im.manifest.Layers {
		if h == desc.Digest {
			return layoutLayer{layout: im.layout, desc: desc}, nil
		}
	}
	return nil, errors.Errorf("blob %s not found in image manifest", h)
}

// layoutLayer is a partial.CompressedLayer backed by a blob of an ociLayout.
type layoutLayer struct {
	layout ociLayout
	desc   v1.Descriptor
}

var _ partial.CompressedLayer = layoutLayer{}

func (l layoutLayer) Digest() (v1.Hash, error) {
	return l.desc.Digest, nil
}

func (l layoutLayer) Compressed() (io.ReadCloser, error) {
	return l.layout.blob(l.desc.Digest)
}

func (l layoutLayer) Size() (int64, error) {
	return l.desc.Size, nil
}

func (l layoutLayer) MediaType() (types.MediaType, error) {
	return l.desc.MediaType, nil
}

// ociLayout resolves blob digests to files under an OCI layout root.
type ociLayout struct {
	root string
}

func (l ociLayout) blob(h v1.Hash) (io.ReadCloser, error) {
	return os.Open(filepath.Join(l.root, "blobs", h.Algorithm, h.Hex))
}

func (l ociLayout) bytes(h v1.Hash) ([]byte, error) {
	return os.ReadFile(filepath.Join(l.root, "blobs", h.Algorithm, h.Hex))
}
