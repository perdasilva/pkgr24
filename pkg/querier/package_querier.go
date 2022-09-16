package querier

import (
	"context"
	"io/fs"
	"strings"
	"time"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/pkg/registry"
	pkgr24iov1alpha1 "github.com/perdasilva/pkgr24/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const catalogFileName = "catalog.json"

type packageQuerier struct {
	k8sClient client.Client
}

func NewPackageQuerier(ctx context.Context, k8sClient client.Client) (*registry.Querier, error) {
	// get all packages
	pkgList := pkgr24iov1alpha1.PackageList{}
	if err := k8sClient.List(ctx, &pkgList); err != nil {
		return nil, err
	}

	// concatenate fbcs
	builder := strings.Builder{}
	for _, pkg := range pkgList.Items {
		builder.WriteString(pkg.Spec.PackageFBC)
	}

	// make declerative config
	cfg, err := declcfg.LoadFile(newStringFS(builder.String()), catalogFileName)
	if err != nil {
		return nil, err
	}

	// make model
	model, err := declcfg.ConvertToModel(*cfg)
	if err != nil {
		return nil, err
	}

	// return querier
	return registry.NewQuerier(model)
}

type stringFileInfo struct {
	name string
	size int64
}

func (f *stringFileInfo) Name() string {
	return f.name
}

func (f *stringFileInfo) Size() int64 {
	return f.size
}
func (f *stringFileInfo) Mode() fs.FileMode {
	return 0664
}
func (f *stringFileInfo) ModTime() time.Time {
	return time.Now()
}
func (f *stringFileInfo) IsDir() bool {
	return false
}
func (f *stringFileInfo) Sys() any {
	return nil
}

type stringFile struct {
	name   string
	reader *strings.Reader
}

func newStringFile(name string, contents string) *stringFile {
	return &stringFile{
		name:   name,
		reader: strings.NewReader(contents),
	}
}

func (f *stringFile) Stat() (fs.FileInfo, error) {
	return &stringFileInfo{
		name: f.name,
		size: int64(f.reader.Len()),
	}, nil
}

func (f *stringFile) Read(b []byte) (int, error) {
	return f.reader.Read(b)
}

func (f *stringFile) Close() error {
	return nil
}

type stringFS struct {
	fbc *stringFile
}

func newStringFS(contents string) *stringFS {
	return &stringFS{
		fbc: newStringFile(catalogFileName, contents),
	}
}

func (fs *stringFS) Open(name string) (fs.File, error) {
	return fs.fbc, nil
}
